#!/usr/bin/python3

import socket
import sys
import os
import getopt
from hardwaretimesource import HardwareTimeSource
from ntptimesource import NTPTimeSource
from datetime import datetime
from dateutil import tz
from datetime import timezone
import systemtime
from dateutil.tz import tzlocal
import signal

class TimeDaemon:
    MAX_DELTA=1.00  # 1s default tolerance

    def __init__(self, hw_time_source, ntp_time_source):
        self.hw_ts=hw_time_source
        self.ntp_ts=ntp_time_source

        hw_time_source.register_gps_time_handler(self._gps_time_received)
        hw_time_source.register_bricklets_discovery_finished(self._bricklets_discovered)

    def _is_similar(self, time1, time2, max_difference=MAX_DELTA):
        delta=(time1-time2).total_seconds()
        delta=abs(delta)
        return (delta<max_difference)

    def _update_system_time(self, new_time):
        new_time=new_time.astimezone(tz=tzlocal())
        systemtime.set_system_time(new_time.timetuple())

    def _bricklets_discovered(self):
        print("All necessary bricklets are discovered!")
        pass

    def _gps_time_received(self, gps_time):
        now=datetime.utcnow().replace(tzinfo=tz.tzutc())
        
        if self._is_similar(gps_time, now):
            # GPS and local time are similar. Update RTC clock
            # and local time from GPS
            self._update_system_time(gps_time)
            self.hw_ts.update_rtc_time(gps_time)
        else:
            print("GPS and system time are different")
            # TODO: gps and local time are significantly different check with other sources
            rtc_time=self.hw_ts.get_rtc_time()
            print("RTC Time: %s" % rtc_time)
            if self._is_similar(gps_time, rtc_time):
                # RTC and GPS times are similar, probably first boot
                self._update_system_time(gps_time)
                self.hw_ts.update_rtc_time(gps_time)
            else:
                print("RTC and GPS time are different, checking NTP")
                # GPS time is out of sync from both local and RTC time
                # We will check it with NTP
                ntp_time, _ = self.ntp_ts.get_ntp_time()
                if self._is_similar(gps_time, ntp_time, max_difference=5): # we have 5s tolerance because of network nature of NTP
                    print("NTP and GPS are similar, trusting GPS")
                    # GPS and NTP are close. RTC battery might be new
                    self._update_system_time(gps_time)
                    self.hw_ts.update_rtc_time(gps_time)
                else:
                    print("PANIC! Unable to reliably determine time! Not setting any time....")
                    # TODO: Sound the buzzer
                    

def signal_handler(signal, frame):
        print("Exiting...")
        sys.exit(0)

if __name__ == "__main__":
    print("Starting time daemon")

    hs=HardwareTimeSource("localhost", "4223")
    ns=NTPTimeSource(["0.pool.ntp.org", "3.ch.pool.ntp.org", "3.europe.pool.ntp.org", "europe.pool.ntp.org"], 5) #TODO: Load ntp servers from config

    td=TimeDaemon(hs, ns)

    signal.signal(signal.SIGINT, signal_handler)
    print('Press Ctrl+C to exit')
    signal.pause()