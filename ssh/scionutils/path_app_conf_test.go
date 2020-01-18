package scionutils

import (
	"github.com/scionproto/scion/go/lib/addr"
	"reflect"
	"testing"

	"github.com/scionproto/scion/go/lib/snet"
)

func TestPathAppConf_PolicyConnFromConfig(t *testing.T) {
	tables := []struct {
		pathConf   PathAppConf
		policyConn PathSelector
	}{
		{PathAppConf{pathSelection: Arbitrary}, &defaultPathSelector{}},
		{PathAppConf{pathSelection: RoundRobin}, &roundRobinPathSelector{}},
		{PathAppConf{pathSelection: Static}, &staticPathSelector{}},
	}

	for _, table := range tables {
		conn, err := table.pathConf.PolicyConnFromConfig(snet.Conn(nil), nil, addr.IA{})
		if err != nil {
			t.Errorf("PolicyConnFromConfig returned an error: %s", err)
		}

		resultType := reflect.TypeOf(conn.(*policyConn).pathSelector)
		expectedType := reflect.TypeOf(table.policyConn)
		if resultType != expectedType {
			t.Errorf("PolicyConnFromConfig expecting path selector type %s, got type %s", expectedType, resultType)
		}
	}
}
