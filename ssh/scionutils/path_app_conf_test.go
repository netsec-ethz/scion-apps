package scionutils

import (
	"reflect"
	"testing"

	"github.com/scionproto/scion/go/lib/snet"
)

func TestPathAppConf_PolicyConnFromConfig(t *testing.T) {
	tables := []struct {
		pathConf   PathAppConf
		policyConn snet.Conn
	}{
		{PathAppConf{pathSelection: Arbitrary}, &policyConn{}},
		{PathAppConf{pathSelection: RoundRobin}, &roundRobinPolicyConn{}},
		{PathAppConf{pathSelection: Static}, &staticPolicyConn{}},
		{PathAppConf{pathSelection: Random}, nil},
	}

	for _, table := range tables {
		conn, err := table.pathConf.PolicyConnFromConfig(snet.Conn(nil))
		if table.pathConf.pathSelection == Random {
			if err == nil {
				t.Errorf("PolicyConnFromConfig instantiated type %s for unsupported Random path selection. "+
					"Expecting an error.", reflect.TypeOf(conn))
			}
		} else {
			if err != nil {
				t.Errorf("PolicyConnFromConfig returned an error: %s", err)
			}

			resultType := reflect.TypeOf(conn)
			expectedType := reflect.TypeOf(table.policyConn)
			if resultType != expectedType {
				t.Errorf("PolicyConnFromConfig expecting type %s, got type %s", expectedType, resultType)
			}
		}

	}
}
