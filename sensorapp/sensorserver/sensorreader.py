#!/usr/bin/env python3
# -*- coding: utf-8 -*-

HOST = "localhost"
PORT = 4223
CO2_UID = "CW1"
TEMPERATURE_UID = "zFn"
SOUND_INTENSITY_UID = "B59"
DUST_UID = "Bt3"
AMBIENTLIGHT_UID = "yFZ"
UVLIGHT_UID = "xn8"
MASTERBRICK1_UID = "5W4zM3"
MASTERBRICK2_UID = "6JMWng"
PIEZOSPEAKER_UID = "C8k"
MOTIONDETECTOR_UID = "BRA"
HUMIDITY_UID = "CXa"

from tinkerforge.ip_connection import IPConnection
from tinkerforge.bricklet_co2 import BrickletCO2
from tinkerforge.bricklet_sound_intensity import SoundIntensity
from tinkerforge.bricklet_dust_detector import DustDetector
from tinkerforge.bricklet_temperature import Temperature
from tinkerforge.bricklet_ambient_light import AmbientLight
from tinkerforge.bricklet_uv_light import BrickletUVLight
from tinkerforge.bricklet_motion_detector import BrickletMotionDetector
from tinkerforge.bricklet_humidity_v2 import HumidityV2

import time
from datetime import datetime

if __name__ == "__main__":
    ipcon = IPConnection() # Create IP connection
    co2 = BrickletCO2(CO2_UID, ipcon) # Create device object
    humidity = HumidityV2(HUMIDITY_UID, ipcon)
    sound_intensity = SoundIntensity(SOUND_INTENSITY_UID, ipcon)
    dust_density = DustDetector( DUST_UID, ipcon)
    temperature = Temperature( TEMPERATURE_UID, ipcon)
    ambientlight = AmbientLight( AMBIENTLIGHT_UID, ipcon)
    uvlight = BrickletUVLight( UVLIGHT_UID, ipcon)
    motiondetect = BrickletMotionDetector( MOTIONDETECTOR_UID, ipcon)

    ipcon.connect(HOST, PORT) # Connect to brickd

    while (True):
        curtime = datetime.now()
        print( "Time: " + curtime.strftime('%Y/%m/%d %H:%M:%S'))

        motion = motiondetect.get_motion_detected()
        print( "Motion: " + str( motion ))

        illuminance = ambientlight.get_illuminance()/10.0
        print( "Illuminance: " + str(illuminance))

        uv_light = uvlight.get_uv_light()
        print( "UV Light: " + str(uv_light))

        # Get current CO2 concentration (unit is ppm)
        cur_co2_concentration = co2.get_co2_concentration()
        print( "CO2: " + str(cur_co2_concentration))

        # Get current sound intensity level
        cur_si = sound_intensity.get_intensity()
        print( "Sound intensity: " + str(cur_si))

        # Get current dust density
        cur_dd = dust_density.get_dust_density()
        print( "Dust density: " + str(cur_dd))

        # Get current humidity level
        cur_humidity = humidity.get_humidity()/100.0
        print("Humidity: " + str(cur_humidity))

        # Get temperature from humidity sensor
        cur_humidity = humidity.get_temperature()/100.0
        print("Temperature (Humidity sensor): " + str(cur_humidity))

        # Temperature
        cur_temp = temperature.get_temperature()/100.00
        print( "Temperature: " + str(cur_temp), flush=True )

        # Print out values every 10 seconds
        time.sleep(10)
