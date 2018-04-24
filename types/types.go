package types

import "encoding/xml"

type Configuration struct {
	XMLName xml.Name  `xml:"configuration"`
	Root    Container `xml:"root"`

	Version      string `xml:"version,attr"`
	SavePassword bool   `xml:"savepassword,attr"`

	//AllConnections map[string]*Connection `xml:"-"`
}

func (c *Configuration) AllConnections() map[string]*Connection {
	//TODO cache
	return ListConnections(c, false)
}

type _node struct {
	Type string `xml:"type,attr"`
	Name string `xml:"name,attr"`

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
	Prompt   string `xml:"prompt"`
}

// Timeouts are in milliseconds
type Timeout struct {
	ConnectionTimeout int `xml:"connectiontimeout"`
	LoginTimeout      int `xml:"logintimeout"`
	PasswordTimeout   int `xml:"passwordtimeout"`
	CommandTimeout    int `xml:"commandtimeout"`
}

type Command struct {
	Command1 string `xml:"command1"`
	Command2 string `xml:"command2"`
	Command3 string `xml:"command3"`
	Command4 string `xml:"command4"`
	Command5 string `xml:"command5"`
}

type Options struct {
	LoginMacro bool `xml:"loginmacro"`
	// Note that the original PCM sends commands no matter what this flag is set to
	PostCommands bool   `xml:"postcommands"`
	EndlineChar  int    `xml:"endlinechar"`
	SSHPublicKey string `xml:"ssh_public_key,omitempty"`
}

// lists all connections of config into a path->connection mapping
func ListConnections(config *Configuration,
	includeDescription bool) map[string]*Connection {

	conns := make(map[string]*Connection)
	descendConnections("", &config.Root, conns, includeDescription)
	return conns
}

// Recursively descend through connections tree, writing paths->connection mappings
// into the conns map. Start with prefix ""
func descendConnections(prefix string, node *Container,
	conns map[string]*Connection, includeDescription bool) {
	node.Path_ = prefix
	for i := range node.Connections {
		c := &node.Connections[i]
		key := prefix + "/" + c.Name
		c.Path_ = key
		if includeDescription {
			key += "  " + c.Info.Description
		}
		conns[key] = c
	}
	for i := range node.Containers {
		n := &node.Containers[i]
		descendConnections(prefix+"/"+n.Name, n, conns,
			includeDescription)
	}
}
