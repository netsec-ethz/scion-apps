package mpsquic

import (
	"fmt"
	"strings"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/spath"
)

// Debug helpers

func printHFDetails(path *spath.Path) {
	vpath := path.Copy()
	vpath.HopOff = common.LineLen // skip InfoField, cannot use vpath.InitOffsets() as it skips more
	fmt.Printf("\nFields:")
	for {
		if vpath.HopOff >= len(vpath.Raw) {
			break
		}
		hf, err := vpath.GetHopField(vpath.HopOff)
		if err != nil {
			fmt.Printf("\n\nGetHopField err:%v", err)
			break
		}
		XoverVal := "."
		if hf.Xover {
			XoverVal = "X"
		}
		VerifyOnlyVal := "."
		if hf.VerifyOnly {
			VerifyOnlyVal = "V"
		}
		fmt.Printf("\n\tHF %s%s InIF: %3v OutIF: %3v \t\t\tExpTime: %v Mac: %v",
			XoverVal, VerifyOnlyVal, hf.ConsIngress, hf.ConsEgress, hf.ExpTime, hf.Mac)
		err = vpath.IncOffsetsRaw(spath.HopFieldLength, false)
		if err != nil {
			if strings.Contains(err.Error(), "Info Field parse error") {
				continue
			}
			fmt.Printf("\n\nIncOffsetsRaw err:%v", err)
			break
		}
	}
	fmt.Println()
}