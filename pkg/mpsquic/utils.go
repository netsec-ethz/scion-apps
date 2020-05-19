package mpsquic

func exportTraces() error {
	/*
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
	*/
	return nil
}
