#!/usr/bin/env python3

from picamera import PiCamera, Color
from time import sleep
from datetime import datetime

camera = PiCamera()

# 0, 90, 180, 270
camera.rotation = 180

# min (64, 64), max (2592, 1944)
# camera.resolution = (2592, 1944)

# max 15
# camera.framerate = 15

# min 0, max 100
camera.brightness = 70

# min 0, max 100
camera.contrast = 50

# min 6, max 160
camera.annotate_text_size = 32
camera.annotate_background = Color( 'black' )
camera.annotate_foreground = Color( 'white' )

while True:
    curtime = datetime.now()
    outstring = curtime.strftime('%Y/%m/%d %H:%M:%S')
    filename = "office-" + curtime.strftime('%Y%m%d-%H:%M:%S') + ".jpg"
    camera.annotate_text = outstring

    # camera.start_preview()
    # sleep(1)
    camera.capture( filename )
    # camera.stop_preview()
    sleep( 120 )

