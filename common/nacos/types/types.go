package types

type NacosConf struct {
	Addr      string // "127.0.0.1:8848" (可扩展成多个)
	Namespace string
	Group     string // DEFAULT_GROUP
	Username  string
	Password  string

	// 服务注册发现
	Cluster string // DEFAULT or "cluster-a"
}
