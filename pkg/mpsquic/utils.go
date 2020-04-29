package mpsquic

import (
	"errors"
	"fmt"
	"os"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/infra/modules/combinator"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/spath"
)

// Parse raw forwarding path spath.Path to combinator.Path .
func parseSPath(vpath spath.Path) (cpath *combinator.Path, err error) {
	var segments []*combinator.Segment
	var interfaces []sciond.PathInterface

	vpath.InfOff = 0
	vpath.HopOff = common.LineLen // skip InfoField, cannot use vpath.InitOffsets() as it skips more

	infoF, err := vpath.GetInfoField(vpath.InfOff)
	if err != nil {
		return nil, err
	}
	for {
		if vpath.HopOff >= len(vpath.Raw) {
			break
		}
		var segment *combinator.Segment

		if vpath.HopOff-vpath.InfOff > int(infoF.Hops)*spath.HopFieldLength {
			// Switch to next segment
			vpath.InfOff = vpath.HopOff
			infoF, err = vpath.GetInfoField(vpath.InfOff)
			if err != nil {
				return nil, err
			}
			vpath.HopOff += common.LineLen
		}

		var hopFields []*combinator.HopField
		var segInterfaces []sciond.PathInterface
		for i := 0; i < int(infoF.Hops); i++ {
			hf, err := vpath.GetHopField(vpath.HopOff)
			if err != nil {
				return nil, err
			}
			vpath.HopOff += spath.HopFieldLength

			hopFields = append(hopFields, &combinator.HopField{hf})
			segInterfaces = append(segInterfaces, sciond.PathInterface{0, hf.ConsIngress})
			segInterfaces = append(segInterfaces, sciond.PathInterface{0, hf.ConsEgress})
		}

		segment = &combinator.Segment{
			InfoField:  &combinator.InfoField{infoF},
			HopFields:  hopFields,
			Type:       0,
			Interfaces: segInterfaces,
		}
		segments = append(segments, segment)
		interfaces = append(interfaces, segInterfaces...)
	}
	if !vpath.IsEmpty() && len(segments) == 0 {
		logger.Error("Invalid raw path length.", "HopOff", vpath.HopOff, "len(Raw)", len(vpath.Raw))
		return nil, errors.New("invalid raw path length")
	}
	return &combinator.Path{
		Segments:   segments,
		Weight:     0,
		Mtu:        0,
		Interfaces: interfaces,
	}, nil
}

// Debug helpers

// printHFDetails prints the HopField metainformation: Ingress/Egress interface, expiration time, MAC and Xover/VerifyOnly properties.
func printHFDetails(path *spath.Path) {
	cpath, err := parseSPath(*path)
	if err != nil {
		logger.Error("Failed to parse path info.", "err", err)
		return
	}
	fmt.Printf("\nFields:")
	for _, s := range cpath.Segments {
		for _, hf := range s.HopFields {
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
		}
	}
	fmt.Println()
}

func exportTraces() error {
	if tracer == nil {
		logger.Trace("No QUIC tracer registered, nothing to export.")
		return nil
	}
	traces := tracer.GetAllTraces()
	i := 0
	for _, trace := range traces {
		f, err := os.Create(fmt.Sprintf("/tmp/mpsquic_trace_%d.qtr", i))
		if err != nil {
			return err
		}
		if _, err := f.Write(trace); err != nil {
			return err
		}
		logger.Debug("Wrote QUIC trace file", "path", f.Name())
		i += 1
	}
	return nil
}
