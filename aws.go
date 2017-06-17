package main

import (
	"regexp"
	"strings"

	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cfstras/go-utils/color"
	"github.com/cfstras/go-utils/fileutil"
	"github.com/cfstras/pcm/types"
)

const (
	awsRootName = "--AWS--"
)

var (
	iniRegex = regexp.MustCompile(`^\s*([^=\s]+)\s*=\s*([^=\s]+)\s*$`)
)

type Inst ec2.Instance

func importAWS(conf *types.Configuration) error {
	cfg := &aws.Config{}
	path := replaceHome("~/.aws/config")
	if e, _ := fileutil.Exists(path); e {
		lines, err := fileutil.ReadLines(path)
		if err != nil {
			color.Yellowln("Could not parse", path, ":", err)
		} else {
			for _, l := range lines {
				if match := iniRegex.FindStringSubmatch(l); match != nil {
					if strings.ToLower(match[1]) == "region" {
						cfg.Region = &match[2]
					}
				}
			}
		}
	}
	sess, err := session.NewSession(cfg)
	if err != nil {
		return err
	}
	ec2 := ec2.New(sess)
	inst, err := ec2.DescribeInstances(nil)
	if err != nil {
		return err
	}
	container := types.Container{}
	container.Name = awsRootName
	for _, r := range inst.Reservations {
		for _, in := range r.Instances {
			inst := Inst(*in)
			conn := types.Connection{}
			conn.Info = types.Info{Name: inst.getTag("Name"),
				Host: *inst.PublicDnsName,
				Port: 22}
			conn.Name = conn.Info.Name
			for i, t := range inst.Tags {
				if i != 0 {
					conn.Info.Description += "; "
				}
				conn.Info.Description += *t.Key + ": " + *t.Value
			}
			conn.Login.User = "centos"
			container.Connections = append(container.Connections, conn)
		}
	}
	sort.Sort(SortConn(container.Connections))
	//TODO order by tags
	conf.Root.Containers = append([]types.Container{container}, conf.Root.Containers...)
	return nil
}
func (inst Inst) getTag(n string) string {
	for _, t := range inst.Tags {
		if *t.Key == n {
			return *t.Value
		}
	}
	return "not found: " + n
}

type SortConn []types.Connection

func (a SortConn) Len() int           { return len(a) }
func (a SortConn) Less(i, j int) bool { return strings.Compare(a[i].Name, a[j].Name) < 0 }
func (a SortConn) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
