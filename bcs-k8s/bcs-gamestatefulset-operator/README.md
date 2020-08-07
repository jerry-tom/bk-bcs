## gamestatefulset-operator

gamestatefulset-operator是针对游戏gameserver实现增强版有状态部署operator。 

### 重构目标

* [done]本项目group重构可用
* [done]增加InplaceUpdate策略
* [done]增加自动并行滚动更新
* [todo]集成腾讯云CLB，实现有状态端口段动态转发
* [todo]集成BCS无损更新特性：允许不重启容器更新容器内容
* [todo]扩展kubectl，支持kubectl gamestatefulset子命令
* [todo]支持HPA

### 特性

基于CRD+Operator开发的一种自定义的K8S工作负载（GameStatefulSet），核心特性包括：

* 兼容StatefulSet所有特性
* 支持Operator高可用部署
* 支持Node失联时，Pod的自动漂移（StatefulSet不支持）  
* 支持容器原地升级

### 信息初始化

初始化依赖信息，安装gamestatefulset-operator

```shell
$ kubectl create -f 01-resources.yaml

$ kubectl create -f 02-namespace.yaml

$ kubectl create -f 03-rbac.yaml

$ kubectl create -f 04-operator-deployment.yaml
```

### 使用案例

* 扩缩容 > kubectl scale --replicas=3 gamestatefulset/web -n test 
* 滚动升级 > kubectl patch 

#### 创建gamestatefulset

```shell
$ kubectl create -f doc/example/gamestatefulset-sample.yml

#check pod status
$ kubectl get pod -n test | grep web 
web-0                              1/1     Running   0         21s
```

#### 扩容gamestatefulset

```shell
$ kubectl scale --replicas=3 gamestatefulset/web -n test 

$ kubectl get pod -n test | grep web 
web-0   1/1     Running   0          2m
web-1   1/1     Running   0          13s
web-2   1/1     Running   0          10s
```

#### InplaceUpdate

```shell
$ kubectl patch gamestatefulset web -n test --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"test.artifactory.com:8090/public/bcs/bcs-loadbalance:v1.2.0"}]'
#或者调整yaml文件之后 kubectl apply -f gamestatefulset-sample.yml

#检查Pod状态，restart增1
$ kubectl get pod -n test
NAME    READY   STATUS    RESTARTS   AGE
web-0   2/2     Running   1          5m17s
web-0   2/2     Running   1          4m30s
web-0   2/2     Running   1          4m27s

#主机上确认更新结果为原地更新，仅重启Pod中变更的容器
$ docker ps | grep web 
86e6e387df1d        5a4aadde608a    "python -m SimpleHTT…"   14 seconds ago      Up 14 seconds                           k8s_python_web-0_test_1439b3f6-4d67-11ea-8202-52540097500a_1
2aaa9ff0acae        3b282bc5e585    "python -m SimpleHTT…"   5 minutes ago       Up 5 minutes                           k8s_sidecar_web-0_test_1439b3f6-4d67-11ea-8202-52540097500a_0
554cfa8aa81e        7fc9ac0fb989    "/pause"                 5 minutes ago       Up 5 minutes                           k8s_POD_web-0_test_1439b3f6-4d67-11ea-8202-52540097500a_0
```
