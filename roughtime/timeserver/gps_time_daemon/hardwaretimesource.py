import time, threading

from datetime import timezone
from datetime import datetime
from dateutil import tz
import calendar

from tinkerforge.ip_connection import IPConnection

from tinkerforge.bricklet_gps_v2 import BrickletGPSV2
from tinkerforge.bricklet_oled_128x64 import BrickletOLED128x64
from tinkerforge.bricklet_real_time_clock import BrickletRealTimeClock

utc_zone = tz.tzutc()

class GpsLocation:
    def __init__(self, latitude, ns, longitude, ew):
        self.latitude = float(latitude)/1000000.0
        self.longitude = float(longitude)/1000000.0
        self.ns = ns
        self.ew = ew

class HardwareTimeSource:
    GPS_UPDATE_PERIOD = 5000
    RTC_UPDATE_PERIOD = 1000

    def __init__(self, host, port):
        # Available devices that we use
        self.gps = None
        self.rtc = None
        self.oled = None
        self.time_handler=None

        # GPS information
        self.last_gps_time = None
        self.last_gps_position = None

        self.ipcon = IPConnection() 
        self.ipcon.register_callback(IPConnection.CALLBACK_ENUMERATE, 
                                     self._cb_enumerate)
        self.ipcon.register_callback(IPConnection.CALLBACK_CONNECTED, 
                                     self._cb_connected)
        self.ipcon.connect(host, int(port))
        self.ipcon.enumerate()

    def _cb_enumerate(self, uid, connected_uid, position, hardware_version, 
                 firmware_version, device_identifier, enumeration_type):
        
        if enumeration_type == IPConnection.ENUMERATION_TYPE_CONNECTED or \
           enumeration_type == IPConnection.ENUMERATION_TYPE_AVAILABLE:
            
            # Initialize GPS
            if device_identifier == BrickletGPSV2.DEVICE_IDENTIFIER:
                self.gps = BrickletGPSV2(uid, self.ipcon)

                self.gps.set_date_time_callback_period(HardwareTimeSource.GPS_UPDATE_PERIOD)
                self.gps.set_coordinates_callback_period(HardwareTimeSource.GPS_UPDATE_PERIOD)

                self.gps.register_callback(BrickletGPSV2.CALLBACK_DATE_TIME, self._cb_time_updated)
                self.gps.register_callback(BrickletGPSV2.CALLBACK_COORDINATES, self._cb_location_updated)

            # Initialize OLED display
            if device_identifier == BrickletOLED128x64.DEVICE_IDENTIFIER:
                self.oled = BrickletOLED128x64(uid, self.ipcon)
                self.oled.clear_display()

            # Initialize RTC
            if device_identifier == BrickletRealTimeClock.DEVICE_IDENTIFIER:
                self.rtc = BrickletRealTimeClock(uid, self.ipcon)

                self.rtc.register_callback(BrickletRealTimeClock.CALLBACK_DATE_TIME, self._cb_rtc_time_update)
                self.rtc.set_date_time_callback_period(HardwareTimeSource.RTC_UPDATE_PERIOD)

        if self.rtc and self.gps and self.ready_handler:
            # We are ready to server time
            self.ready_handler()

    def _cb_connected(self, connected_reason):
        self.ipcon.enumerate()

    def _cb_time_updated(self, d, t):
        fix, satelite_num = self.gps.get_status()
        if fix:
            year, d = d % 100, int(d/100)
            month, d = d % 100, int(d/100)
            day = d % 100

            millisecond, t= t % 1000, int(t/1000)
            second, t = t % 100, int(t/100)
            minute, t = t % 100, int(t/100)
            hour = t % 100

            self.last_gps_time = datetime(2000+year, month, day, hour, minute, second, microsecond=millisecond*1000, tzinfo=utc_zone)

            if self.time_handler:
                self.time_handler(self.last_gps_time)

            if self.oled:
                self.oled.write_line(3, 2, "GPS Time: %02d:%02d:%02d.%02d" % (self.last_gps_time.hour, self.last_gps_time.minute, self.last_gps_time.second, millisecond/10))
                self.oled.write_line(4, 2, "GPS Date: %02d.%02d.%d" % (self.last_gps_time.day, self.last_gps_time.month, self.last_gps_time.year))

    def _cb_location_updated(self, latitude, ns, longitude, ew):
        fix, satelite_num = self.gps.get_status()
        if fix:
            self.last_gps_position=GpsLocation(latitude, ns, longitude, ew)
            if self.oled:
                self.oled.write_line(6, 1, "Location: %.2f %s %.2f %s" % (self.last_gps_position.latitude, ns, self.last_gps_position.longitude, ew))

    def _cb_rtc_time_update(self, year, month, day, hour, minute, second, centisecond, weekday, timestamp):
        if self.oled:
            self.oled.write_line(0, 2, "RTC Time: %02d:%02d:%02d.%02d" % (hour, minute, second, centisecond))
            self.oled.write_line(1, 2, "RTC Date: %02d.%02d.%d" % (day, month, year))

    def register_gps_time_handler(self, gps_time_handler):
        self.time_handler=gps_time_handler

    def register_bricklets_discovery_finished(self, ready_handler):
        self.ready_handler=ready_handler

    def get_rtc_time(self):
        if self.rtc:
            year, month, day, hour, minute, second, centisecond, weekday = self.rtc.get_date_time()
            dt = datetime(year, month, day, hour, minute, second, centisecond*10000, tzinfo=utc_zone)
            return dt
        else:
            raise Exception("RTC is not initialized!")

    def update_rtc_time(self, dt):
        if self.rtc:
            # TODO: Check if dt has timezone set to UTC, throw exception otherwise
            self.rtc.set_date_time(dt.year, dt.month, dt.day, dt.hour, dt.minute, dt.second, dt.microsecond/10000, dt.weekday()+1)
        else:
            raise Exception("RTC is not initialized!")