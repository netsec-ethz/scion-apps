#!/usr/bin/python3
# -*- coding: utf-8 -*-

import socket
import time
from datetime import datetime
from socket import gethostname

last_idle = last_total = 0


if __name__ == "__main__":

    while (True):

        curtime = datetime.now()
        print("Time: " + curtime.strftime('%Y/%m/%d %H:%M:%S'))

        print("Hostname: " + gethostname())

        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.connect(("8.8.8.8", 80))
        print("IP: " + s.getsockname()[0])

        with open('/proc/stat') as f:
            fields = [float(column)
                      for column in f.readline().strip().split()[1:]]
        idle, total = fields[3], sum(fields)
        idle_delta, total_delta = idle - last_idle, total - last_total
        last_idle, last_total = idle, total
        utilization = 100.0 * (1.0 - idle_delta / total_delta)
        print('CPU Utilization: %.1f%%' % utilization, flush=True)

        time.sleep(10)
