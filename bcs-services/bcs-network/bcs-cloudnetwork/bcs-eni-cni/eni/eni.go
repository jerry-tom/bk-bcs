/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.,
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under,
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package eni

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"bk-bcs/bcs-common/common/blog"
	"bk-bcs/bcs-common/common/conf"
	bcsconf "bk-bcs/bcs-services/bcs-netservice/config"
	"bk-bcs/bcs-services/bcs-network/bcs-cloudnetwork/pkg/constant"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

var defaultLogDir = "./logs"

// NetConf net config
type NetConf struct {
	types.NetConf
	MTU    int    `json:"mtu,omitempty"`
	LogDir string `json:"logDir,omitempty"`
	Args   *bcsconf.CNIArgs
}

func loadConf(bytes []byte, args string) (*NetConf, string, error) {
	conf := &NetConf{}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, "", fmt.Errorf("failed to load cni conf, err %s", err.Error())
	}
	if conf.MTU < 68 || conf.MTU > 65535 {
		blog.Errorf("invalid mtu %d", conf.MTU)
		return nil, "", fmt.Errorf("invalid mtu %d", conf.MTU)
	}
	if len(conf.LogDir) == 0 {
		blog.Errorf("log dir is empty, use default log dir './logs'")
		conf.LogDir = defaultLogDir
	}
	if args != "" {
		conf.Args = &bcsconf.CNIArgs{}
		err := types.LoadArgs(args, conf.Args)
		if err != nil {
			return nil, "", err
		}
	}
	return conf, conf.CNIVersion, nil
}

// ENI cni object
type ENI struct{}

// New create ENI object
func New() *ENI {
	return &ENI{}
}

// getRouteTableIDByMac get route table id by mac address and eni prefix
func getRouteTableIDByMac(mac, eniPrefix string) (int, error) {
	links, err := netlink.LinkList()
	if err != nil {
		blog.Errorf("list links failed, err %s", err.Error())
		return -1, fmt.Errorf("list links failed, err %s", err.Error())
	}
	for _, l := range links {
		if strings.ToLower(l.Attrs().HardwareAddr.String()) == strings.ToLower(mac) {
			if !strings.HasPrefix(l.Attrs().Name, eniPrefix) {
				blog.Errorf("eni with mac %s does not has prefix %s", mac, eniPrefix)
				return -1, fmt.Errorf("eni with mac %s does not has prefix %s", mac, eniPrefix)
			}
			idString := strings.Trim(l.Attrs().Name, eniPrefix)
			id, err := strconv.Atoi(idString)
			if err != nil {
				blog.Errorf("convert %s to int failed, err %s", idString, err.Error())
				return -1, fmt.Errorf("convert %s to int failed, err %s", idString, err.Error())
			}
			return id + constant.START_ROUTE_TABLE, nil
		}
	}
	return -1, fmt.Errorf("cannot find eni with mac %s", mac)
}

// createVethPair create veth pair, return with cni format
func createVethPair(netns string, containerIfName string, mtu int) (*current.Interface, *current.Interface, error) {
	containerIface := &current.Interface{}
	hostIface := &current.Interface{}

	// create veth pair in container ns
	if err := ns.WithNetNSPath(netns, func(hostNS ns.NetNS) error {
		hostVeth, containerVeth, err := ip.SetupVeth(containerIfName, mtu, hostNS)
		if err != nil {
			return err
		}
		containerIface.Name = containerVeth.Name
		containerIface.Mac = containerVeth.HardwareAddr.String()
		containerIface.Sandbox = netns
		hostIface.Name = hostVeth.Name
		return nil
	}); err != nil {
		return nil, nil, err
	}

	hostVeth, err := netlink.LinkByName(hostIface.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup %q: %v", hostIface.Name, err)
	}
	hostIface.Mac = hostVeth.Attrs().HardwareAddr.String()
	return hostIface, containerIface, nil
}

// configureHostNS configure host namespace
func configureHostNS(hostIfName string, ipNet *net.IPNet, routeTableID int) error {

	// add to taskgroup route
	hostVeth, err := netlink.LinkByName(hostIfName)
	if err != nil {
		return fmt.Errorf("failed to look up %s in host ns, err %s", hostIfName, err.Error())
	}
	// add route in certain route table
	route := &netlink.Route{
		LinkIndex: hostVeth.Attrs().Index,
		Scope:     netlink.SCOPE_LINK,
		Dst:       ipNet,
		Table:     routeTableID,
	}
	err = netlink.RouteAdd(route)
	if err != nil {
		return fmt.Errorf("add route %s into host failed, err %s", route.String(), err.Error())
	}

	//add to taskgroup rule
	//**attention** do not usage &netlink.Rule{} for struct initialization
	ruleToTable := netlink.NewRule()
	ruleToTable.Dst = ipNet
	ruleToTable.Table = routeTableID
	err = netlink.RuleDel(ruleToTable)
	if err != nil {
		blog.Warnf("clean old rule to table %s failed, err %s", ruleToTable.String(), err.Error())
	}
	err = netlink.RuleAdd(ruleToTable)
	if err != nil {
		return fmt.Errorf("add rule to table %s failed, err %s", ruleToTable.String(), err.Error())
	}

	//add from taskgroup rule
	ruleFromTaskgroup := netlink.NewRule()
	ruleFromTaskgroup.Src = ipNet
	ruleFromTaskgroup.Table = routeTableID
	err = netlink.RuleDel(ruleFromTaskgroup)
	if err != nil {
		blog.Warnf("clean old rule from taskgroup %s failed, err %s", ruleToTable.String(), err.Error())
	}
	err = netlink.RuleAdd(ruleFromTaskgroup)
	if err != nil {
		return fmt.Errorf("add rule from taskgroup %s failed, err %s", ruleFromTaskgroup.String(), err.Error())
	}

	return nil
}

// configureContainerNS configure container namespace
// 1. set address for veth in container namespace
// 2. add routes
// 3. set static arp
func configureContainerNS(hostMac, netns, containerIfName string, ipNet *net.IPNet, gw net.IP) error {
	if err := ns.WithNetNSPath(netns, func(hostNS ns.NetNS) error {
		containerVeth, err := netlink.LinkByName(containerIfName)
		if err != nil {
			return fmt.Errorf("failed to look up %s in ns %s, err %s", containerIfName, netns, err.Error())
		}
		netlink.AddrAdd(containerVeth, &netlink.Addr{IPNet: ipNet})

		gwNet := &net.IPNet{IP: gw, Mask: net.CIDRMask(32, 32)}

		if err = netlink.RouteAdd(&netlink.Route{
			LinkIndex: containerVeth.Attrs().Index,
			Scope:     netlink.SCOPE_LINK,
			Dst:       gwNet,
		}); err != nil {
			return fmt.Errorf("add route to %v in ns %s failed, err %s", gwNet.String(), netns, err.Error())
		}

		defaultRoute := netlink.Route{
			LinkIndex: containerVeth.Attrs().Index,
			Dst:       &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},
			Scope:     netlink.SCOPE_UNIVERSE,
			Gw:        gw,
			Src:       ipNet.IP,
		}
		if err = netlink.RouteAdd(&defaultRoute); err != nil {
			return fmt.Errorf("add default route in ns %s failed, err %s", netns, err.Error())
		}

		hostHardwareAddr, err := net.ParseMAC(hostMac)
		if err != nil {
			return fmt.Errorf("parse mac from %s failed, err %s", hostMac, err.Error())
		}
		neigh := &netlink.Neigh{
			LinkIndex:    containerVeth.Attrs().Index,
			State:        netlink.NUD_PERMANENT,
			IP:           gwNet.IP,
			HardwareAddr: hostHardwareAddr,
		}

		if err = netlink.NeighAdd(neigh); err != nil {
			return fmt.Errorf("setup NS network: failed to add static ARP, err %s", err.Error())
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// CNIAdd cni cmd add
func (e *ENI) CNIAdd(args *skel.CmdArgs) error {
	netConf, cniVersion, err := loadConf(args.StdinData, args.Args)
	if err != nil {
		return fmt.Errorf("load config stdindata %s, args %s failed, err %s",
			string(args.StdinData), args.Args, err.Error())
	}

	blog.InitLogs(conf.LogConfig{
		LogDir: netConf.LogDir,
		// never log to stderr
		StdErrThreshold: "6",
		LogMaxSize:      20,
		LogMaxNum:       100,
	})
	defer blog.CloseLogs()

	// get ip address from ipam plugin
	resultFromIPAM, err := ipam.ExecAdd(netConf.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}
	blog.Infof("get result from ipam %s", resultFromIPAM.String())

	result, err := current.NewResultFromResult(resultFromIPAM)
	if err != nil {
		return err
	}
	if len(result.IPs) == 0 {
		blog.Errorf("IPAM plugin %s returned missing IP config", result.String())
		return fmt.Errorf("IPAM plugin %s returned missing IP config", result.String())
	}
	if len(result.Interfaces) == 0 {
		blog.Errorf("IPAM plugin %s returned missing mac addr info", result.String())
		return fmt.Errorf("IPAM plugin %s returned missing mac addr info", result.String())
	}

	ipNet := &net.IPNet{
		IP:   result.IPs[0].Address.IP,
		Mask: net.IPv4Mask(255, 255, 255, 255),
	}
	eniMac := result.Interfaces[0].Mac

	// find eni id according to eniMac
	routeTableID, err := getRouteTableIDByMac(eniMac, constant.ENI_PREFIX)
	if err != nil {
		blog.Errorf("get route table id by mac %s with eni prefix %s failed, err %s", eniMac, constant.ENI_PREFIX, err.Error())
		return fmt.Errorf("get route table id by mac %s with eni prefix %s failed, err %s", eniMac, constant.ENI_PREFIX, err.Error())
	}

	// get container namespace
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		blog.Errorf("failed to get netns %q, err %s", netns, err.Error())
		return fmt.Errorf("failed to get netns %q, err %s", netns, err.Error())
	}

	hostVethInfo, containerVethInfo, err := createVethPair(netns.Path(), args.IfName, netConf.MTU)
	if err != nil {
		blog.Errorf("create veth pair failed, err %s", err.Error())
		return fmt.Errorf("create veth pair failed, err %s", err.Error())
	}
	blog.Infof("get hostVeth %v, containerVeth %v", hostVethInfo, containerVethInfo)

	err = configureContainerNS(hostVethInfo.Mac, netns.Path(), args.IfName, ipNet, result.IPs[0].Gateway)
	if err != nil {
		blog.Errorf("configure container ns network failed, err %s", err.Error())
		return fmt.Errorf("configure container ns network failed, err %s", err.Error())
	}

	err = configureHostNS(hostVethInfo.Name, ipNet, routeTableID)
	if err != nil {
		blog.Errorf("configure host ns network failed, err %s", err.Error())
		return fmt.Errorf("configure host ns network failed, err %s", err.Error())
	}

	contIndex := 1
	ips := []*current.IPConfig{
		&current.IPConfig{
			Version:   "4",
			Address:   *ipNet,
			Interface: &contIndex,
		},
	}

	result = &current.Result{
		IPs:        ips,
		Interfaces: []*current.Interface{hostVethInfo, containerVethInfo},
	}

	return types.PrintResult(result, cniVersion)
}

// CNIDel cni cmd del
func (e *ENI) CNIDel(args *skel.CmdArgs) error {

	netConf, _, err := loadConf(args.StdinData, args.Args)
	if err != nil {
		return fmt.Errorf("load config file failed, err %s", err.Error())
	}
	blog.InitLogs(conf.LogConfig{
		LogDir: netConf.LogDir,
		// never log to stderr
		StdErrThreshold: "6",
		LogMaxSize:      20,
		LogMaxNum:       100,
	})
	defer blog.CloseLogs()

	if args.Netns == "" {
		blog.Warnf("Netns lost in parameter")
		return nil
	}

	blog.Infof("received cni del command: containerid %s, netns %s, ifname %s, args %s, path %s argsStdinData %s",
		args.ContainerID, args.Netns, args.IfName, args.Args, args.Path, args.StdinData)

	err = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
	if err != nil {
		blog.Errorf("call IPAM delete function failed, err %s", err.Error())
		return fmt.Errorf("call IPAM delete function failed, err %s", err.Error())
	}

	var addrsToRelease []netlink.Addr
	var hostVethIndex int
	err = ns.WithNetNSPath(args.Netns, func(netNS ns.NetNS) error {
		link, err := netlink.LinkByName(args.IfName)
		if err != nil {
			blog.Errorf("get link by name %s in ns %s failed, err %s", args.IfName, args.Netns, err.Error())
			return fmt.Errorf("get link by name %s in ns %s failed, err %s", args.IfName, args.Netns, err.Error())
		}
		veth, ok := link.(*netlink.Veth)
		if !ok {
			blog.Errorf("link %s is not veth peer, failed", veth.Name)
			return fmt.Errorf("link %s is not veth peer, failed", veth.Name)
		}
		hostVethIndex, err = netlink.VethPeerIndex(veth)
		if err != nil {
			blog.Errorf("failed to get host veth peer index, err %s", err.Error())
			return fmt.Errorf("failed to get host veth peer index, err %s", err.Error())
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			blog.Errorf("get link %s addresses in ns %s failed, err %s", args.IfName, args.Netns, err.Error())
			return fmt.Errorf("get link %s addresses in ns %s failed, err %s", args.IfName, args.Netns, err.Error())
		}
		if len(addrs) == 0 {
			blog.Errorf("get link %s zero addresses in ns %s", args.IfName, args.Netns)
			return fmt.Errorf("get link %s zero addresses in ns %s", args.IfName, args.Netns)
		}
		addrsToRelease = addrs

		// shut down container veth
		_, err = ip.DelLinkByNameAddr(args.IfName, netlink.FAMILY_ALL)
		if err != nil && err == ip.ErrLinkNotFound {
			blog.Errorf("delete link %s failed, err %s", args.IfName, err.Error())
			return nil
		}

		return nil
	})
	if err != nil {
		blog.Errorf("tear ns %s failed, err %s", args.Netns, err.Error())
		return fmt.Errorf("tear ns %s failed, err %s", args.Netns, err.Error())
	}

	// try to delete link in host network namespace
	hostLink, err := netlink.LinkByIndex(hostVethIndex)
	if err != nil {
		blog.Infof("net link with index %d already be deleted, err %s", hostVethIndex, err.Error())
	} else {
		blog.Infof("delete net link %s in host ns", hostLink.Attrs().Name)
		err := netlink.LinkDel(hostLink)
		if err != nil {
			blog.Errorf("failed to delete net link %s in host ns, err %s", hostLink.Attrs().Name, err.Error())
			return fmt.Errorf("failed to delete net link %s in host ns, err %s", hostLink.Attrs().Name, err.Error())
		}
	}

	// delete rule about pod
	for _, addr := range addrsToRelease {
		blog.Infof("delete addr %s route", addr.IPNet.Network())
		ipNet := &net.IPNet{
			IP:   addr.IPNet.IP,
			Mask: net.IPv4Mask(255, 255, 255, 255),
		}
		toTaskgroupRule := netlink.NewRule()
		toTaskgroupRule.Dst = ipNet
		err := netlink.RuleDel(toTaskgroupRule)
		if err != nil {
			blog.Warnf("delete to taskgroup rule %s failed, err %s", toTaskgroupRule.String(), err.Error())
		}
		fromTaskgroupRule := netlink.NewRule()
		fromTaskgroupRule.Src = ipNet
		err = netlink.RuleDel(fromTaskgroupRule)
		if err != nil {
			blog.Warnf("delete from taskgroup rule %s failed, err %s", fromTaskgroupRule.String(), err.Error())
		}
		blog.Infof("delete rules about %s complete", addr)
	}
	return nil
}
