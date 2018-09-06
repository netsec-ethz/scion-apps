import threading
import ntplib
from time import ctime
from datetime import datetime
from dateutil import tz
from dateutil.tz import tzlocal

def query_ntp_server(server_url, request_timeout, result_handler):
    client = ntplib.NTPClient()
    try:
        response = client.request(server_url, version=3, timeout=request_timeout)
        result_handler(response)
    except:
        print("Error getting time from ntp server: %s" % (server_url))

class TimeResult:
    def __init__(self):
        self.responses=[]
        self.response_lock=threading.Lock()

    def response_received(self, response):
        self.response_lock.acquire()
        self.responses.append(response)
        self.response_lock.release()

    def get_time(self):
        times=[]

        for r in self.responses:
            times.append(r.tx_time)

        return self._find_max_window_time(times)

    def _find_max_window_time(self, obtained_times):
        """ We take time that has largest number of occurrences, within delta """
        if len(obtained_times)==0:
            return 0, 0

        obtained_times.sort()

        start=0
        end=0
        max_window=0
        t=0

        while end<len(obtained_times):
            delta=obtained_times[end]-obtained_times[start]
            if delta <= NTPTimeSource.MAX_DELTA_SEC:
                end=end+1
                window_size=end-start
                if window_size>max_window:
                    max_window=window_size
                    t=obtained_times[start]
            else:
                start=start+1

        return t, max_window


class NTPTimeSource:
    MAX_DELTA_SEC=2

    def __init__(self, ntp_servers, request_timeout):
        self.servers=ntp_servers
        self.timeout=request_timeout

    def get_ntp_time(self):
        """ Query all ntp servers in different threads, take the time that has most occurrences """
        response=TimeResult()
        workers=[]

        for server in self.servers:
            t=threading.Thread(target=query_ntp_server, args=(server, self.timeout, response.response_received, ))
            t.start()
            workers.append(t)

        for worker in workers:
            worker.join()

        timestamp, server_num = response.get_time()
        return datetime.fromtimestamp(timestamp).replace(tzinfo=tzlocal()), server_num

if __name__ == "__main__":
    print("Sending query to NTP servers")
    ntp_servers=["0.pool.ntp.org", "3.ch.pool.ntp.org", "3.europe.pool.ntp.org", "europe.pool.ntp.org"]

    ntp_source=NTPTimeSource(ntp_servers, 5)
    t, server_num = ntp_source.get_ntp_time()
    print(t)
