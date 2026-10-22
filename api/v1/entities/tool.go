package entities

type AddToolReq struct {
	Name string `json:"name"` // 名称
	Kind int    `json:"kind"` // 类型。0：local tool; 2: a2a; 3: mcp; 4: skill
	Url  string `json:"url"`  // 远程服务地址，压缩包下载地址
}
