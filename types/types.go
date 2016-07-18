package types

import "encoding/xml"

type Configuration struct {
	XMLName xml.Name  `xml:"configuration"`
	Root    Container `xml:"root"`

	AllConnections map[string]*Connection `xml:"-"`
}

type _node struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`

	Path_      string `xml:"-"`
	TreeView   string `xml:"-"`
	StatusInfo string `xml:"-"`
}

type Node interface {
	Path() string
}

type Container struct {
	_node

	Expanded    bool         `xml:"expanded,attr"`
	Containers  []Container  `xml:"container"`
	Connections []Connection `xml:"connection"`
}

type Connection struct {
	_node

	Expanded bool    `xml:"-"`
	Info     Info    `xml:"connection_info"`
	Login    Login   `xml:"login"`
	Timeout  Timeout `xml:"timeout"`
	Commands Command `xml:"command"`
	Options  Options `xml:"options"`
}

func (n *_node) Path() string {
	return n.Path_
}

type Info struct {
	Name        string `xml:"name"`
	Protocol    string `xml:"protocol"`
	Host        string `xml:"host"`
	Port        uint16 `xml:"port"`
	Session     string `xml:"session"`
	Commandline string `xml:"commandline"`
	Description string `xml:"description"`
}

type Login struct {
	User     string `xml:"login"`
	Password string `xml:"password"`
}

// Timeouts are in milliseconds
type Timeout struct {
	ConnectionTimeout int `xml:"connectiontimeout"`
	LoginTimeout      int `xml:"logintimeout"`
	PasswordTimeout   int `xml:"passwordtimeout"`
	CommandTimeout    int `xml:"commandtimeout"`
}

type Command struct {
	Commands []string `xml:",any"`
}

type Options struct {
	LoginMacros  bool   `xml:"loginmacros"`
	PostCommands bool   `xml:"postCommands"`
	EndlineChar  int    `xml:"endlinechar"`
	SSHPublicKey string `xml:"ssh_public_key"`
}
