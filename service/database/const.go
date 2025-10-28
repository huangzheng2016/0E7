package database

import (
	"time"
)

// Client 客户端信息表
type Client struct {
	ID        int       `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	Name      string    `json:"name" gorm:"column:name;type:varchar(255);not null;default:'';index;"`
	Hostname  string    `json:"hostname" gorm:"column:hostname;type:varchar(255);not null;default:'';index;"`
	Platform  string    `json:"platform" gorm:"column:platform;type:varchar(255);not null;default:'';"`
	Arch      string    `json:"arch" gorm:"column:arch;type:varchar(255);not null;default:'';"`
	CPU       string    `json:"cpu" gorm:"column:cpu;type:varchar(255);not null;default:'';"`
	CPUUse    string    `json:"cpu_use" gorm:"column:cpu_use;type:varchar(255);not null;default:'';"`
	MemoryUse string    `json:"memory_use" gorm:"column:memory_use;type:varchar(255);not null;default:'';"`
	MemoryMax string    `json:"memory_max" gorm:"column:memory_max;type:varchar(255);not null;default:'';"`
	Pcap      string    `json:"pcap" gorm:"column:pcap;type:varchar(255);not null;default:'';"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index;"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime;index;"`
}

func (Client) TableName() string {
	return "0e7_client"
}

// Exploit 漏洞利用表
type Exploit struct {
	ID          int    `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	Name        string `json:"name" gorm:"column:name;type:varchar(255);not null;index;"`
	Filename    string `json:"filename" gorm:"column:filename;type:varchar(255);"`
	Environment string `json:"environment" gorm:"column:environment;type:varchar(255);"`
	Command     string `json:"command" gorm:"column:command;type:varchar(255);"`
	Argv        string `json:"argv" gorm:"column:argv;type:varchar(255);"`
	Platform    string `json:"platform" gorm:"column:platform;type:varchar(255);index;"`
	Arch        string `json:"arch" gorm:"column:arch;type:varchar(255);index;"`
	Filter      string `json:"filter" gorm:"column:filter;type:varchar(255);"`
	Timeout     string `json:"timeout" gorm:"column:timeout;type:varchar(255);"`
	Times       string `json:"times" gorm:"column:times;type:varchar(255);not null;default:'0';index;"`
	Flag        string `json:"flag" gorm:"column:flag;type:varchar(255);"`
	Team        string `json:"team" gorm:"column:team;type:varchar(255);index;"`
	IsDeleted   bool   `json:"is_deleted" gorm:"column:is_deleted;type:boolean;default:false;index;"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index;"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Exploit) TableName() string {
	return "0e7_exploit"
}

// Flag 标志表
type Flag struct {
	ID          int       `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	ExploitId   int       `json:"exploit_id" gorm:"column:exploit_id;type:int;not null;default:0;index;"`
	Team        string    `json:"team" gorm:"column:team;type:varchar(255);not null;default:'';index;"`
	Flag        string    `json:"flag" gorm:"column:flag;type:varchar(255);not null;default:'';index;"`
	Status      string    `json:"status" gorm:"column:status;type:varchar(255);index;"`
	Msg         string    `json:"msg" gorm:"column:msg;type:text;"` // 提交结果消息
	ExploitName string    `json:"exploit_name" gorm:"-"`            // 不存储到数据库，仅用于显示
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;index;"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Flag) TableName() string {
	return "0e7_flag"
}

// ExploitOutput 漏洞利用输出表
type ExploitOutput struct {
	ID        int       `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	ExploitId int       `json:"exploit_id" gorm:"column:exploit_id;type:int;not null;default:0;index;"`
	ClientId  int       `json:"client_id" gorm:"column:client_id;type:int;not null;default:0;index;"`
	Output    string    `json:"output" gorm:"column:output;type:text;"`
	Status    string    `json:"status" gorm:"column:status;type:varchar(255);index;"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index;"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ExploitOutput) TableName() string {
	return "0e7_exploit_output"
}

// Action 动作表
type Action struct {
	ID        int       `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	Name      string    `json:"name" gorm:"column:name;type:varchar(255);not null;default:'';index;"`
	Code      string    `json:"code" gorm:"column:code;type:text;"`
	Output    string    `json:"output" gorm:"column:output;type:text;"`
	Error     string    `json:"error" gorm:"column:error;type:text;"`
	Config    string    `json:"config" gorm:"column:config;type:text;"`
	Interval  int       `json:"interval" gorm:"column:interval;type:int;"`
	Timeout   int       `json:"timeout" gorm:"column:timeout;type:int;default:60;"`                             // 超时时间（秒），默认60秒，最多60秒
	Status    string    `json:"status" gorm:"column:status;type:varchar(50);default:'pending';not null;index;"` // 任务状态：pending, running, completed, timeout, error
	NextRun   time.Time `json:"next_run" gorm:"column:next_run;type:datetime;index;"`                           // 下次执行时间
	IsDeleted bool      `json:"is_deleted" gorm:"column:is_deleted;type:boolean;default:false;index;"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index;"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Action) TableName() string {
	return "0e7_action"
}

// PcapFile PCAP文件表
type PcapFile struct {
	ID        int       `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	Filename  string    `json:"filename" gorm:"column:filename;type:varchar(255);index;"`
	ModTime   time.Time `json:"mod_time" gorm:"column:mod_time;type:datetime;index;"`
	FileSize  int64     `json:"file_size" gorm:"column:file_size;type:bigint;"`
	MD5       string    `json:"md5" gorm:"column:md5;type:varchar(32);index;"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index;"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PcapFile) TableName() string {
	return "0e7_pcapfile"
}

// Monitor 监控表
type Monitor struct {
	ID        int       `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	ClientId  int       `json:"client_id" gorm:"column:client_id;type:int;not null;default:0;index;"`
	Name      string    `json:"name" gorm:"column:name;type:varchar(255);index;"`
	Types     string    `json:"types" gorm:"column:types;type:varchar(255);index;"`
	Data      string    `json:"data" gorm:"column:data;type:text;"`
	Interval  int       `json:"interval" gorm:"column:interval;type:int;"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index;"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Monitor) TableName() string {
	return "0e7_monitor"
}

// Pcap PCAP数据表
type Pcap struct {
	ID            int       `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	SrcPort       string    `json:"src_port" gorm:"column:src_port;type:varchar(255);index;"`
	DstPort       string    `json:"dst_port" gorm:"column:dst_port;type:varchar(255);index;"`
	SrcIP         string    `json:"src_ip" gorm:"column:src_ip;type:varchar(255);index;"`
	DstIP         string    `json:"dst_ip" gorm:"column:dst_ip;type:varchar(255);index;"`
	Time          int       `json:"time" gorm:"column:time;type:int;index;"`
	Duration      int       `json:"duration" gorm:"column:duration;type:int;"`
	NumPackets    int       `json:"num_packets" gorm:"column:num_packets;type:int;"`
	Blocked       string    `json:"blocked" gorm:"column:blocked;type:varchar(255);index;"`
	Filename      string    `json:"filename" gorm:"column:filename;type:varchar(255);index;"`
	FlowFile      string    `json:"flow_file" gorm:"column:flow_file;type:text;"`               // 大文件路径（前端通过有无判断是否需要点击加载）
	FlowData      string    `json:"flow_data,omitempty" gorm:"column:flow_data;type:longtext;"` // 小文件时的JSON字符串（前端自己解析）
	PcapFile      string    `json:"pcap_file" gorm:"column:pcap_file;type:varchar(255);"`       // pcap文件路径
	PcapData      string    `json:"-" gorm:"column:pcap_data;type:longblob;"`                   // 不返回（小pcap数据，base64编码）
	Tags          string    `json:"tags" gorm:"column:tags;type:text;index;"`
	ClientContent string    `json:"-" gorm:"column:client_content;type:text;index;"` // 不返回（用于搜索）
	ServerContent string    `json:"-" gorm:"column:server_content;type:text;index;"` // 不返回（用于搜索）
	Size          int       `json:"size" gorm:"column:size;type:int;"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime;index;"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Pcap) TableName() string {
	return "0e7_pcap"
}
