#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import time
from datetime import datetime

if __name__ == "__main__":
    while (True):
        curtime = datetime.now()
        print( "Time: " + curtime.strftime('%Y/%m/%d %H:%M:%S'), flush=True )
        time.sleep(1)
