package database

import (
	"time"
)

// Client 客户端信息表
type Client struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	UUID      string    `json:"uuid" gorm:"column:uuid;type:varchar(255);not null;"`
	Hostname  string    `json:"hostname" gorm:"column:hostname;type:varchar(255);not null;"`
	Platform  string    `json:"platform" gorm:"column:platform;type:varchar(255);not null;"`
	Arch      string    `json:"arch" gorm:"column:arch;type:varchar(255);not null;"`
	CPU       string    `json:"cpu" gorm:"column:cpu;type:varchar(255);not null;"`
	CPUUse    string    `json:"cpu_use" gorm:"column:cpu_use;type:varchar(255);not null;"`
	MemoryUse string    `json:"memory_use" gorm:"column:memory_use;type:varchar(255);not null;"`
	MemoryMax string    `json:"memory_max" gorm:"column:memory_max;type:varchar(255);not null;"`
	Pcap      string    `json:"pcap" gorm:"column:pcap;type:varchar(255);not null;"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Client) TableName() string {
	return "0e7_client"
}

// Exploit 漏洞利用表
type Exploit struct {
	ID          uint      `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	UUID        string    `json:"uuid" gorm:"column:uuid;type:varchar(255);not null;"`
	Filename    string    `json:"filename" gorm:"column:filename;type:varchar(255);"`
	Environment string    `json:"environment" gorm:"column:environment;type:varchar(255);"`
	Command     string    `json:"command" gorm:"column:command;type:varchar(255);"`
	Argv        string    `json:"argv" gorm:"column:argv;type:varchar(255);"`
	Platform    string    `json:"platform" gorm:"column:platform;type:varchar(255);"`
	Arch        string    `json:"arch" gorm:"column:arch;type:varchar(255);"`
	Filter      string    `json:"filter" gorm:"column:filter;type:varchar(255);"`
	Timeout     string    `json:"timeout" gorm:"column:timeout;type:varchar(255);"`
	Times       string    `json:"times" gorm:"column:times;type:varchar(255);not null;"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Exploit) TableName() string {
	return "0e7_exploit"
}

// Flag 标志表
type Flag struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	UUID      string    `json:"uuid" gorm:"column:uuid;type:varchar(255);not null;"`
	Flag      string    `json:"flag" gorm:"column:flag;type:varchar(255);not null;"`
	Status    string    `json:"status" gorm:"column:status;type:varchar(255);"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Flag) TableName() string {
	return "0e7_flag"
}

// ExploitOutput 漏洞利用输出表
type ExploitOutput struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	UUID      string    `json:"uuid" gorm:"column:uuid;type:varchar(255);not null;"`
	Client    string    `json:"client" gorm:"column:client;type:varchar(255);not null;"`
	Output    string    `json:"output" gorm:"column:output;type:text;"`
	Status    string    `json:"status" gorm:"column:status;type:varchar(255);"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ExploitOutput) TableName() string {
	return "0e7_exploit_output"
}

// Action 动作表
type Action struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	Name      string    `json:"name" gorm:"column:name;type:varchar(255);not null;unique;"`
	Code      string    `json:"code" gorm:"column:code;type:text;"`
	Output    string    `json:"output" gorm:"column:output;type:text;"`
	Interval  int       `json:"interval" gorm:"column:interval;type:int;"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Action) TableName() string {
	return "0e7_action"
}

// PcapFile PCAP文件表
type PcapFile struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	Filename  string    `json:"filename" gorm:"column:filename;type:varchar(255);"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PcapFile) TableName() string {
	return "0e7_pcapfile"
}

// Monitor 监控表
type Monitor struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	UUID      string    `json:"uuid" gorm:"column:uuid;type:varchar(255);"`
	Types     string    `json:"types" gorm:"column:types;type:varchar(255);"`
	Data      string    `json:"data" gorm:"column:data;type:text;"`
	Interval  int       `json:"interval" gorm:"column:interval;type:int;"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Monitor) TableName() string {
	return "0e7_monitor"
}

// Pcap PCAP数据表
type Pcap struct {
	ID           uint      `json:"id" gorm:"column:id;primary_key;auto_increment;"`
	SrcPort      string    `json:"src_port" gorm:"column:src_port;type:varchar(255);"`
	DstPort      string    `json:"dst_port" gorm:"column:dst_port;type:varchar(255);"`
	SrcIP        string    `json:"src_ip" gorm:"column:src_ip;type:varchar(255);"`
	DstIP        string    `json:"dst_ip" gorm:"column:dst_ip;type:varchar(255);"`
	Time         int       `json:"time" gorm:"column:time;type:int;"`
	Duration     int       `json:"duration" gorm:"column:duration;type:int;"`
	NumPackets   int       `json:"num_packets" gorm:"column:num_packets;type:int;"`
	Blocked      string    `json:"blocked" gorm:"column:blocked;type:varchar(255);"`
	Filename     string    `json:"filename" gorm:"column:filename;type:varchar(255);"`
	Fingerprints string    `json:"fingerprints" gorm:"column:fingerprints;type:text;"`
	Suricata     string    `json:"suricata" gorm:"column:suricata;type:text;"`
	Flow         string    `json:"flow" gorm:"column:flow;type:text;"`
	Tags         string    `json:"tags" gorm:"column:tags;type:text;"`
	Size         string    `json:"size" gorm:"column:size;type:varchar(255);"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Pcap) TableName() string {
	return "0e7_pcap"
}
