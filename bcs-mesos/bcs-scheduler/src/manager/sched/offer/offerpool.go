/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package offer

import (
	"bk-bcs/bcs-common/common/blog"
	typesplugin "bk-bcs/bcs-common/common/plugin"
	commtype "bk-bcs/bcs-common/common/types"
	"bk-bcs/bcs-mesos/bcs-scheduler/src/mesosproto/mesos"
	"bk-bcs/bcs-mesos/bcs-scheduler/src/types"
	"bk-bcs/bcs-mesos/bcs-scheduler/src/util"
	"container/list"
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"strconv"
	"sync"
	"time"
)

const (
	// lost mesos slave's grace period time
	DefaultLostSlaveGracePeriod = 180

	//mesos slave offer life period
	DefaultOfferLifePeriod = 240

	//decline offer grace period
	DefaultDeclineOfferGracePeriod = 5

	DeclineOfferChannelLength = 1024
	DefaultOfferEventLength   = 1024
	DefaultMiniResourceCpu    = 0.05
)

type offerPool struct {
	sync.RWMutex

	offerList *list.List
	offerIds  map[string]bool

	decOffers chan *mesos.Offer

	autoIncrementId int64

	slaveLock  sync.RWMutex
	lostSlaves map[string]int64

	offerEvents chan []*mesos.Offer

	scheduler SchedManager

	lostSlaveGracePeriod int
	offerLifePeriod      int
	declineOfferPeriod   int

	cxt    context.Context
	cancel context.CancelFunc
}

// new struct OfferPool
func NewOfferPool(para *OfferPara) OfferPool {
	o := &offerPool{
		offerList:       list.New(),
		offerIds:        make(map[string]bool, 0),
		decOffers:       make(chan *mesos.Offer, DeclineOfferChannelLength),
		autoIncrementId: 1,
		scheduler:       para.Sched,
		lostSlaves:      make(map[string]int64, 0),
		offerEvents:     make(chan []*mesos.Offer, DefaultOfferEventLength),
	}

	if para.LostSlaveGracePeriod > 0 {
		o.lostSlaveGracePeriod = para.LostSlaveGracePeriod
	} else {
		o.lostSlaveGracePeriod = DefaultLostSlaveGracePeriod
	}

	if para.OfferlifePeriod > 0 {
		o.offerLifePeriod = para.OfferlifePeriod
	} else {
		o.offerLifePeriod = DefaultOfferLifePeriod
	}

	o.declineOfferPeriod = o.offerLifePeriod + DefaultDeclineOfferGracePeriod
	o.cxt, o.cancel = context.WithCancel(context.Background())

	o.start()
	return o
}

type innerOffer struct {
	id       int64
	offerId  string
	hostname string

	isValid     bool
	createdTime int64

	offer *mesos.Offer

	deltaCPU  float64
	deltaMem  float64
	deltaDisk float64
}

func (p *offerPool) start() {
	go p.checkOffers()

	go p.handleDeclineOffers()

	go p.handleOfferEvents()
}

func (p *offerPool) stop() {
	p.cancel()
}

//the implements of interface OfferPool's function AddOffers
func (p *offerPool) AddOffers(offers []*mesos.Offer) error {
	p.offerEvents <- offers

	return nil
}

//the implements of interface OfferPool's function GetOffersLength
func (p *offerPool) GetOffersLength() int {
	p.RLock()
	length := p.offerList.Len()
	p.RUnlock()

	return length
}

//the implements of interface OfferPool's function GetFirstOffer
func (p *offerPool) GetFirstOffer() *Offer {
	p.RLock()
	var offer *Offer

	head := p.offerList.Front()
	for {
		if head == nil {
			break
		}

		innerOffer := head.Value.(*innerOffer)
		blog.V(3).Infof("getFirstOffer offer(%s:%s) isValid %t", innerOffer.offerId,
			innerOffer.hostname, innerOffer.isValid)

		if innerOffer.isValid {
			offer = &Offer{
				element:   head,
				Offer:     innerOffer.offer,
				Id:        innerOffer.id,
				offerId:   innerOffer.offerId,
				hostname:  innerOffer.hostname,
				DeltaCPU:  innerOffer.deltaCPU,
				DeltaMem:  innerOffer.deltaMem,
				DeltaDisk: innerOffer.deltaDisk,
			}
			break
		}

		blog.V(3).Infof("getFirstOffer offer(%s:%s) get next offer",
			innerOffer.offerId, innerOffer.hostname)
		head = head.Next()
	}

	if offer != nil {
		blog.V(3).Infof("GetFirstOffer offer(%s:%s)", offer.offerId, offer.hostname)
	} else {
		blog.Infof("GetFirstOffer offer pool don't have offers")
	}

	p.RUnlock()
	return offer
}

//the implements of interface OfferPool's function GetNextOffer
func (p *offerPool) GetNextOffer(o *Offer) *Offer {
	p.RLock()
	defer p.RUnlock()
	blog.V(3).Infof("GetNextOffer id offer(%d | %s:%s)'s next offer", o.Id, o.offerId, o.hostname)

	if p.offerList.Len() == 0 {
		blog.Infof("GetNextOffer offer(%s:%s) pool don't have offers",
			o.offerId, o.hostname)
		return nil
	}

	var offer *Offer

	_, ok := p.offerIds[o.offerId]
	if ok {
		blog.V(3).Infof("getNextOffer offer(%s:%s) exist", o.offerId, o.hostname)

		head := o.element.Next()
		for {
			if head == nil {
				break
			}

			innerOffer := head.Value.(*innerOffer)
			blog.V(3).Infof("getNextOffer offer(%s:%s) isValid %t", innerOffer.offerId,
				innerOffer.hostname, innerOffer.isValid)

			if innerOffer.isValid {
				offer = &Offer{
					element:   head,
					Offer:     innerOffer.offer,
					Id:        innerOffer.id,
					offerId:   innerOffer.offerId,
					hostname:  innerOffer.hostname,
					DeltaCPU:  innerOffer.deltaCPU,
					DeltaMem:  innerOffer.deltaMem,
					DeltaDisk: innerOffer.deltaDisk,
				}
				break
			}

			blog.V(3).Infof("getNextOffer offer(%s:%s) get next offer",
				innerOffer.offerId, innerOffer.hostname)
			head = head.Next()
		}

		if offer != nil {
			blog.V(3).Infof("GetNextOffer offer(%d | %s:%s)", offer.Id, offer.offerId, offer.hostname)
		} else {
			blog.Infof("GetNextOffer offer(%s:%s) don't have next offer",
				o.offerId, o.hostname)
		}

		return offer
	}

	blog.V(3).Infof("getNextOffer offer(%s:%s) don't exist", o.offerId, o.hostname)
	head := p.offerList.Front()
	for {
		if head == nil {
			break
		}

		innerOffer := head.Value.(*innerOffer)
		blog.V(3).Infof("getNextOffer id %d offer(%s:%s) isValid %t",
			innerOffer.id, innerOffer.offerId, innerOffer.hostname, innerOffer.isValid)

		if innerOffer.isValid {
			offer = &Offer{
				element:   head,
				Offer:     innerOffer.offer,
				Id:        innerOffer.id,
				offerId:   innerOffer.offerId,
				hostname:  innerOffer.hostname,
				DeltaCPU:  innerOffer.deltaCPU,
				DeltaMem:  innerOffer.deltaMem,
				DeltaDisk: innerOffer.deltaDisk,
			}

			blog.V(3).Infof("GetNextOffer offer(%d:%s)", offer.Id, offer.hostname)
			return offer

		}

		head = head.Next()
	}

	blog.V(3).Infof("GetNextOffer offer(%s:%s) don't have next offer", o.offerId, o.hostname)

	return nil
}

//the implements of interface OfferPool's function GetAllOffers
func (p *offerPool) GetAllOffers() []*Offer {
	p.RLock()
	defer p.RUnlock()

	offers := make([]*Offer, 0)

	head := p.offerList.Front()
	for {
		if head == nil {
			break
		}

		innerOffer := head.Value.(*innerOffer)
		if innerOffer.isValid {
			offer := &Offer{
				element:   head,
				Offer:     innerOffer.offer,
				Id:        innerOffer.id,
				offerId:   innerOffer.offerId,
				hostname:  innerOffer.hostname,
				DeltaCPU:  innerOffer.deltaCPU,
				DeltaMem:  innerOffer.deltaMem,
				DeltaDisk: innerOffer.deltaDisk,
			}
			blog.V(3).Infof("GetAllOffers offer(%d:%s)", offer.Id, offer.hostname)
			offers = append(offers, offer)
		}

		head = head.Next()
	}

	return offers
}

//the implements of interface OfferPool's function GetOfferGreaterThan
/*func (p *offerPool) GetOfferGreaterThan(id int64) *Offer {
	p.RLock()
	var offer *Offer
	blog.V(3).Infof("to get offer ( id > %d )", id)

	head := p.offerList.Front()
	for {
		if head == nil {
			break
		}

		innerOffer := head.Value.(*innerOffer)
		blog.V(3).Infof("get offer(%d | %s:%s) isValid %t",
			innerOffer.id, innerOffer.offerId, innerOffer.hostname, innerOffer.isValid)

		if innerOffer.isValid && innerOffer.id > id {
			offer = &Offer{
				element:   head,
				Offer:     innerOffer.offer,
				Id:        innerOffer.id,
				offerId:   innerOffer.offerId,
				hostname:  innerOffer.hostname,
				DeltaCPU:  innerOffer.deltaCPU,
				DeltaMem:  innerOffer.deltaMem,
				DeltaDisk: innerOffer.deltaDisk,
			}
			break
		}

		blog.V(3).Infof("get next offer of offer(%s:%s)", innerOffer.offerId, innerOffer.hostname)
		head = head.Next()
	}

	if offer != nil {
		blog.V(3).Infof("get offer(%d | %s:%s)", offer.Id, offer.offerId, offer.hostname)
	} else {
		blog.V(3).Infof("offer pool don't have proper offer for id > %d", id)
	}
	p.RUnlock()
	return offer
}*/

//the implements of interface OfferPool's function UseOffer
func (p *offerPool) UseOffer(o *Offer) bool {
	p.Lock()
	defer p.Unlock()

	_, ok := p.offerIds[o.offerId]
	if !ok {
		blog.Warnf("use offer(%d | %s:%s), but not found", o.Id, o.offerId, o.hostname)
		return false
	}

	p.offerList.Remove(o.element)
	delete(p.offerIds, o.offerId)

	blog.Infof("after use offer(%d | %s:%s), offers num(%d)", o.Id, o.offerId, o.hostname, p.offerList.Len())
	return true
}

//the implements of interface OfferPool's function AddLostSlave
func (p *offerPool) AddLostSlave(hostname string) {
	p.slaveLock.Lock()
	blog.Infof("slave %s lost", hostname)
	p.lostSlaves[hostname] = -1
	p.slaveLock.Unlock()

	p.Lock()
	p.deleteOfferByHostname(hostname)
	blog.Infof("after delete lost offers from %s, offers num(%d)", hostname, p.offerList.Len())
	p.Unlock()
}

func (p *offerPool) handleOfferEvents() {
	for {
		select {
		case <-p.cxt.Done():
			blog.Warnf("offerPool stop handleOfferEvents")
			return
		case offers := <-p.offerEvents:
			p.addOffers(offers)
		}
	}
}

func (p *offerPool) addOffers(offers []*mesos.Offer) bool {
	p.Lock()
	defer p.Unlock()

	blog.Infof("before add offers, offers num(%d)", p.offerList.Len())
	//sort.Sort(offerSorter(offers))
	for _, o := range offers {
		cpu, mem, disk, port := p.offeredResources(o)

		blog.Infof("add offer(%s:%s) cpu %f mem %f disk %f port %s",
			o.GetId().GetValue(), o.GetHostname(), cpu, mem, disk, port)

		exist, elem := p.slaveIsExist(o.GetHostname())
		if exist {
			blog.Infof("offer from %s exist, decline all offers from the slave", o.GetHostname())
			p.declineOffer(o)
			p.deleteOfferElement(elem)
			continue
		}

		ok := p.validateOffer(o)
		if !ok {
			p.declineOffer(o)
			blog.Warnf("validateOffer offer(%s:%s) is false", o.GetId().GetValue(), o.GetHostname())
			continue
		} else {
			blog.V(3).Infof("validateOffer offer(%s:%s) is ok", o.GetId().GetValue(), o.GetHostname())
		}
		//set offer cpuset resources
		p.setOfferCpuset(o)
		//set offer attributes
		p.setOffersAttributes([]*mesos.Offer{o})
		//print offer info
		p.printOffer(o)
		//add agent deltaXXX 20180530
		agentSchedInfo, err := p.scheduler.FetchAgentSchedInfo(o.GetHostname())
		if err != nil {
			blog.Errorf("get agent(%s) err(%s), offer can not added", o.GetHostname(), err.Error())
			continue
		}
		agentDeltaCPU := 0.0
		agentDeltaMem := 0.0
		agentDeltaDisk := 0.0
		if agentSchedInfo != nil {
			agentDeltaCPU = agentSchedInfo.DeltaCPU
			agentDeltaMem = agentSchedInfo.DeltaMem
			agentDeltaDisk = agentSchedInfo.DeltaDisk
			blog.V(3).Infof("get agent(%s) delta(cpu: %f | mem: %f | disk: %f)",
				o.GetHostname(), agentDeltaCPU, agentDeltaMem, agentDeltaDisk)
		}

		off := &innerOffer{
			id:          p.autoIncrementId,
			offerId:     o.GetId().GetValue(),
			hostname:    o.GetHostname(),
			isValid:     true,
			createdTime: time.Now().Unix(),
			offer:       o,
			deltaCPU:    agentDeltaCPU,
			deltaMem:    agentDeltaMem,
			deltaDisk:   agentDeltaDisk,
		}
		p.autoIncrementId++

		p.pushOfferBySort(off)
		p.offerIds[off.offerId] = true
	}

	blog.Infof("after add offers, offers num(%d)", p.offerList.Len())

	return true
}

func (p *offerPool) setOfferCpuset(offer *mesos.Offer) error {
	ip, ok := GetOfferIp(offer)
	if !ok {
		return nil
	}
	agent, err := p.scheduler.FetchMesosAgent(ip)
	if err != nil {
		return err
	}
	//key = cpu index, example: 1
	usedCpu := make(map[string]struct{})
	for _, executorId := range offer.GetExecutorIds() {
		//offer executorid is taskgroupid
		podId := executorId.GetValue()
		taskg, err := p.scheduler.FetchTaskGroup(podId)
		if err != nil {
			blog.Errorf("FetchTaskGroup pod %s error %s", podId, err.Error())
			continue
		}
		if taskg.Status != types.TASKGROUP_STATUS_RUNNING && taskg.Status != types.TASKGROUP_STATUS_STAGING &&
			taskg.Status != types.TASKGROUP_STATUS_STARTING {
			continue
		}
		for _, task := range taskg.Taskgroup {
			//this task have cpuset setting
			if len(task.DataClass.Cpuset) != 0 {
				blog.V(3).Infof("node %s task %s use cpuset %v", ip, task.ID, task.DataClass.Cpuset)
				for _, set := range task.DataClass.Cpuset {
					usedCpu[set] = struct{}{}
				}
			}
		}
	}

	//create offer cpuset resources
	cpu, _, _, _ := util.ParseMesosResources(agent.AgentInfo.GetTotalResources())
	cpusetK := "cpuset"
	cpusetT := mesos.Value_SET
	cpuset := &mesos.Resource{
		Name: &cpusetK,
		Type: &cpusetT,
		Set: &mesos.Value_Set{
			Item: make([]string, 0),
		},
	}
	for i := 0; i < int(cpu); i++ {
		_, ok := usedCpu[strconv.Itoa(i)]
		if !ok {
			cpuset.Set.Item = append(cpuset.Set.Item, strconv.Itoa(i))
		}
	}

	offer.Resources = append(offer.Resources, cpuset)
	blog.Infof("offer %s cpuset resources(%s)", ip, cpuset.Set.Item)
	return nil
}

func (p *offerPool) pushOfferBySort(offer *innerOffer) {
	if p.offerList.Len() == 0 {
		p.offerList.PushBack(offer)
		return
	}
	cpu, _, _, _ := p.offeredResources(offer.offer)

	cur := p.offerList.Front()
	for {
		if cur == nil {
			p.offerList.PushBack(offer)
			return
		}

		curOffer := cur.Value.(*innerOffer)
		curCpu, _, _, _ := p.offeredResources(curOffer.offer)

		if curCpu < cpu {
			p.offerList.InsertBefore(offer, cur)
			return
		}

		cur = cur.Next()
	}
}

func (p *offerPool) deleteOfferElement(elem *list.Element) {
	offer := elem.Value.(*innerOffer)

	p.offerList.Remove(elem)
	delete(p.offerIds, offer.offerId)
	blog.Infof("offer(%d | %s:%s) is deleted", offer.id, offer.offerId, offer.hostname)
	p.declineOffer(offer.offer)
}

func (p *offerPool) slaveIsExist(hostname string) (bool, *list.Element) {

	head := p.offerList.Front()
	for {
		if head == nil {
			break
		}

		offer := head.Value.(*innerOffer)
		if offer.hostname == hostname {
			return true, head
		}

		head = head.Next()
	}

	return false, nil
}

func (p *offerPool) deleteOfferByHostname(hostname string) {
	head := p.offerList.Front()
	for {
		if head == nil {
			return
		}

		offer := head.Value.(*innerOffer)
		if offer.hostname == hostname {
			p.deleteOfferElement(head)
			return
		}

		head = head.Next()
	}

}

func (p *offerPool) setOffersAttributes(offers []*mesos.Offer) {
	p.setInnerOffersAttributes(offers)
	err := p.setOfferOuterAttributes(offers)
	if err != nil {
		blog.Errorf("set offers attributes error %s", err.Error())
	}
}

func (p *offerPool) setInnerOffersAttributes(offers []*mesos.Offer) {
	for _, offer := range offers {
		blog.V(3).Infof("offer(%s:%s) is setted inner attributes", offer.GetId().GetValue(), offer.GetHostname())

		ip, ok := p.getOfferIp(offer)
		if !ok {
			blog.Warnf("offer(%s:%s) don't have innerip", offer.GetId().GetValue(), offer.GetHostname())
			continue
		}

		setting, err := p.scheduler.FetchAgentSetting(ip)
		if err != nil {
			blog.Errorf("FetchAgentSetting ip %s error %s", ip, err.Error())
			continue
		}

		if setting == nil {
			blog.V(3).Infof("FetchAgentSetting ip %s is nil", ip)
			continue
		}

		err = p.addOfferAttributes(offer, setting)
		if err != nil {
			blog.Errorf("offer(%s:%s) addOfferAttributes error %s", offer.GetId().GetValue(), offer.GetHostname(), err.Error())
		}
	}
}

func (p *offerPool) checkOffers() {
	tick := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-p.cxt.Done():
			blog.Info("offerPool stop check offers")
			return

		case <-tick.C:
			now := time.Now().Unix()
			p.Lock()
			blog.V(3).Infof("checkOffers offer pool have %d offers", p.offerList.Len())

			head := p.offerList.Front()
			for {

				if head == nil {
					break
				}

				offer := head.Value.(*innerOffer)
				cpu, mem, disk, port := p.offeredResources(offer.offer)
				blog.V(3).Infof("checkOffers offer(%d | %s:%s) cpu %f mem %f disk %f port %s",
					offer.id, offer.offerId, offer.hostname, cpu, mem, disk, port)
				//p.printOffer(offer.offer)

				if offer.isValid && offer.createdTime+int64(p.offerLifePeriod) <= now {
					blog.Infof("offer(%d | %s:%s) is invalid", offer.id, offer.offerId, offer.hostname)
					offer.isValid = false
				}

				if !offer.isValid && offer.createdTime+int64(p.declineOfferPeriod) <= now {
					blog.Infof("offer(%d | %s:%s) is over life period", offer.id, offer.offerId, offer.hostname)
					delElem := head
					head = head.Next()
					p.deleteOfferElement(delElem)
				} else {
					head = head.Next()
				}
			}

			blog.V(3).Infof("after check offer, offers num(%d)", p.offerList.Len())
			p.Unlock()
		}
	}
}

func (p *offerPool) handleDeclineOffers() {
	for {
		select {
		case <-p.cxt.Done():
			blog.Info("offerPool stop check offers")
			return

		case offer := <-p.decOffers:
			_, err := p.scheduler.DeclineResource(offer.Id.Value)
			if err != nil {
				blog.Errorf("decline offer(%s:%s) error %s", offer.GetId().GetValue(), offer.GetHostname(), err.Error())
				/*time.Sleep(time.Second)
				p.declineOffer(offer)*/
			} else {
				blog.Infof("decline offer(%s:%s) success", offer.GetId().GetValue(), offer.GetHostname())
			}
		}
	}
}

func (p *offerPool) declineOffer(offer *mesos.Offer) {
	p.decOffers <- offer
}

func (p *offerPool) printOffer(offer *mesos.Offer) {
	attributes := offer.GetAttributes()
	if attributes != nil {
		blog.Infof("offer(%s:%s) has %d attributes", offer.GetId().GetValue(), offer.GetHostname(), len(attributes))
		for i, attribute := range attributes {

			if attribute.GetType() == mesos.Value_SCALAR {
				blog.Infof("offer %s=> attribute[%d](name:%s type:%d scalar:%f)",
					offer.GetHostname(), i, attribute.GetName(), attribute.GetType(), attribute.Scalar.GetValue())
			} else if attribute.GetType() == mesos.Value_RANGES {
				for _, one := range attribute.Ranges.GetRange() {
					blog.Infof("offer %s=> attribute[%d](name:%s type:%d range:%d-->%d)",
						offer.GetHostname(), i, attribute.GetName(), attribute.GetType(), one.GetBegin(), one.GetEnd())
				}
			} else if attribute.GetType() == mesos.Value_SET {
				for _, item := range attribute.Set.GetItem() {
					blog.Infof("offer %s=> attribute[%d](name:%s type:%d set: %s)",
						offer.GetHostname(), i, attribute.GetName(), attribute.GetType(), item)
				}
			} else if attribute.GetType() == mesos.Value_TEXT {
				blog.Infof("offer %s=> attribute[%d](name:%s type:%d text:%s)",
					offer.GetHostname(), i, attribute.GetName(), attribute.GetType(), attribute.Text.GetValue())
			}
		}
	} else {
		blog.Infof("offer(%s:%s) attributes is nil", offer.GetId().GetValue(), offer.GetHostname())
	}

	return
}

func (p *offerPool) addOfferAttributes(offer *mesos.Offer, agentSetting *commtype.BcsClusterAgentSetting) error {

	if agentSetting == nil {
		return nil
	}

	//customized definition agent setting
	for k, v := range agentSetting.AttrStrings {
		blog.V(3).Infof("offer(%s:%s) add attribute(%s:%s) from agentsetting",
			offer.GetId().GetValue(), offer.GetHostname(), k, v)
		var attr mesos.Attribute
		key := k
		value := v

		blog.V(3).Infof("offer(%s:%s) add attribute(%s:%s) from agentsetting",
			offer.GetId().GetValue(), offer.GetHostname(), k, v)

		attr.Name = &key
		var attrType mesos.Value_Type = mesos.Value_TEXT
		attr.Type = &attrType
		var attrValue mesos.Value_Text
		attrValue.Value = &value.Value
		attr.Text = &attrValue
		offer.Attributes = append(offer.Attributes, &attr)
	}
	//customized definition agent setting
	for k, v := range agentSetting.AttrScalars {
		blog.V(3).Infof("offer(%s:%s) add attribute(%s:%f) from agentsetting",
			offer.GetId().GetValue(), offer.GetHostname(), k, v)
		var attr mesos.Attribute
		key := k
		value := v

		attr.Name = &key
		var attrType mesos.Value_Type = mesos.Value_SCALAR
		attr.Type = &attrType
		var attrValue mesos.Value_Scalar
		attrValue.Value = &value.Value
		attr.Scalar = &attrValue
		offer.Attributes = append(offer.Attributes, &attr)
	}

	//noSchedule, likes k8s Taints\Tolerations
	name := types.MesosAttributeNoSchedule
	t := mesos.Value_SET
	noScheduleAttr := &mesos.Attribute{
		Name: &name,
		Type: &t,
		Set: &mesos.Value_Set{
			Item: make([]string, 0),
		},
	}
	for k, v := range agentSetting.NoSchedule {
		blog.V(3).Infof("offer(%s:%s) add noSchedule attribute(%s:%s) from agentsetting",
			offer.GetId().GetValue(), offer.GetHostname(), k, v)
		if k == "" || v == "" {
			continue
		}
		noScheduleAttr.Set.Item = append(noScheduleAttr.Set.Item, fmt.Sprintf("%s=%s", k, v))
	}
	if len(noScheduleAttr.Set.Item) > 0 {
		offer.Attributes = append(offer.Attributes, noScheduleAttr)
	}

	return nil
}

/*type offerSorter []*mesos.Offer

func (s offerSorter) Len() int      { return len(s) }
func (s offerSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s offerSorter) Less(i, j int) bool {

	cpus_i := 0.0
	cpus_j := 0.0
	for _, res := range s[i].GetResources() {
		if res.GetName() == "cpus" {
			cpus_i += *res.GetScalar().Value
		}
	}
	for _, res := range s[i].GetResources() {
		if res.GetName() == "cpus" {
			cpus_j += *res.GetScalar().Value
		}
	}

	if cpus_i != cpus_j {
		return cpus_i > cpus_j
	}
	mem_i := 0.0
	mem_j := 0.0
	for _, res := range s[i].GetResources() {
		if res.GetName() == "mem" {
			mem_i += *res.GetScalar().Value
		}
	}
	for _, res := range s[j].GetResources() {
		if res.GetName() == "mem" {
			mem_j += *res.GetScalar().Value
		}
	}

	return mem_i > mem_j
}*/

func (p *offerPool) getOfferIp(offer *mesos.Offer) (string, bool) {
	attributes := offer.GetAttributes()
	ip := ""
	ok := false

	for _, attribute := range attributes {
		if attribute.GetName() == "InnerIP" {
			ip = attribute.Text.GetValue()
			ok = true
			break
		}
	}

	if !ok {
		blog.Infof("offer(%s:%s) don't have attribute InnerIP", offer.GetId().GetValue(), offer.GetHostname())
	}

	return ip, ok
}

func (p *offerPool) setOfferOuterAttributes(offers []*mesos.Offer) error {
	/*if p.scheduler.GetPluginManager() == nil {
		blog.V(3).Infof("pluginManager is nil")
		return nil
	}*/

	ips := make([]string, 0)

	for _, offer := range offers {

		ip, ok := p.getOfferIp(offer)
		if ok {
			ips = append(ips, ip)
		}
	}

	para := &typesplugin.HostPluginParameter{
		Ips:       ips,
		ClusterId: p.scheduler.GetClusterId(),
	}

	outerAttri, err := p.scheduler.GetHostAttributes(para)
	if err != nil {
		return err
	}

	by, _ := json.Marshal(outerAttri)
	blog.V(3).Infof("offer outer attributes %s", string(by))

	for _, offer := range offers {
		ip, ok := p.getOfferIp(offer)

		if !ok {
			blog.Warnf("offer(%s:%s) don't have attribute InnerIP",
				offer.GetId().GetValue(), offer.GetHostname())
			continue
		}

		attr, ok := outerAttri[ip]
		if !ok {
			blog.Errorf("offer(%s:%s) don't have outer attributes",
				offer.GetId().GetValue(), offer.GetHostname())
			continue
		}

		setting := p.outerAttributes2Agentsetting(attr)

		err = p.addOfferAttributes(offer, setting)
		if err != nil {
			blog.Errorf("offer(%s:%s) add attributes error %s",
				offer.GetId().GetValue(), offer.GetHostname(), err.Error())
		}
	}

	return nil
}

func (p *offerPool) outerAttributes2Agentsetting(attrs *typesplugin.HostAttributes) *commtype.BcsClusterAgentSetting {
	setting := &commtype.BcsClusterAgentSetting{
		InnerIP:     attrs.Ip,
		AttrStrings: make(map[string]commtype.MesosValue_Text),
		AttrScalars: make(map[string]commtype.MesosValue_Scalar),
	}

	for _, attr := range attrs.Attributes {

		switch attr.Type {
		case typesplugin.ValueScalar:
			scalar := commtype.MesosValue_Scalar{
				Value: attr.Scalar.Value,
			}
			setting.AttrScalars[attr.Name] = scalar

		case typesplugin.ValueText:
			text := commtype.MesosValue_Text{
				Value: attr.Text.Text,
			}
			setting.AttrStrings[attr.Name] = text

		default:
			blog.Errorf("slave outer attributes type %s in invalid", attr.Type)
		}
	}

	return setting
}

func (p *offerPool) validateLostslave(hostname string) bool {
	p.slaveLock.Lock()
	defer p.slaveLock.Unlock()

	t, exist := p.lostSlaves[hostname]
	if !exist {
		return true
	}

	if t == -1 {
		p.lostSlaves[hostname] = time.Now().Unix()
		return false
	}

	if time.Now().Unix()-t <= int64(p.lostSlaveGracePeriod) {
		return false
	}

	delete(p.lostSlaves, hostname)

	return true
}

func (p *offerPool) validateOffer(offer *mesos.Offer) bool {
	hostname := offer.GetHostname()

	ok := p.validateMiniResourceOffer(offer)
	if !ok {
		blog.V(3).Infof("offer(%s:%s) validateMiniResourceOffer is invalid",
			offer.GetId().GetValue(), hostname)
		return false
	}

	ok = p.validateLostslave(hostname)
	if !ok {
		blog.Infof("offer(%s:%s) validateLostslave is invalid",
			offer.GetId().GetValue(), hostname)
		return false
	}

	ok = p.validateDisableSlave(offer)
	if !ok {
		blog.Infof("offer(%s:%s) validateDisableSlave is invalid",
			offer.GetId().GetValue(), hostname)
		return false
	}

	return true
}

func (p *offerPool) validateMiniResourceOffer(offer *mesos.Offer) bool {
	cpu, _, _, _ := p.offeredResources(offer)

	if cpu <= DefaultMiniResourceCpu {
		return false
	}

	return true
}

func (p *offerPool) validateDisableSlave(offer *mesos.Offer) bool {
	ip, ok := p.getOfferIp(offer)
	if !ok {
		return true
	}

	setting, err := p.scheduler.FetchAgentSetting(ip)
	if err != nil {
		blog.Errorf("FetchAgentSetting ip %s error %s", ip, err.Error())
		return true
	}

	if setting == nil {
		blog.V(3).Infof("FetchAgentSetting ip %s is nil", ip)
		return true
	}

	if setting.Disabled {
		blog.Info("host(%s:%s) already disabled, decline offer from it",
			offer.GetId().GetValue(), offer.GetHostname(), ip)
		return false
	}

	return true
}

func (p *offerPool) offeredResources(offer *mesos.Offer) (cpus, mem, disk float64, port string) {
	for _, res := range offer.GetResources() {
		if res.GetName() == "cpus" {
			cpus += *res.GetScalar().Value
		}
		if res.GetName() == "mem" {
			mem += *res.GetScalar().Value
		}
		if res.GetName() == "disk" {
			disk += *res.GetScalar().Value
		}
		if res.GetName() == "ports" {
			port = res.GetRanges().String()
		}
	}

	return
}

func GetOfferAttribute(offer *mesos.Offer, name string) (*mesos.Attribute, error) {

	attributes := offer.GetAttributes()
	if attributes == nil {
		blog.V(3).Infof("offer from host(%s) attributes == nil", offer.GetHostname())
		return nil, nil
	}

	for _, attribute := range attributes {
		if attribute.GetName() == name {
			blog.V(3).Infof("offer from host(%s) attribute(%s) setted", offer.GetHostname(), name)
			return attribute, nil
		}
	}

	blog.V(3).Infof("offer from host(%s) attribute(%s) unsetted", offer.GetHostname(), name)
	return nil, nil
}

func GetOfferIp(offer *mesos.Offer) (string, bool) {
	attributes := offer.GetAttributes()
	ip := ""
	ok := false

	for _, attribute := range attributes {
		if attribute.GetName() == "InnerIP" {
			ip = attribute.Text.GetValue()
			ok = true
			break
		}
	}

	if !ok {
		blog.Infof("offer(%s:%s) don't have attribute InnerIP", offer.GetId().GetValue(), offer.GetHostname())
	}

	return ip, ok
}
