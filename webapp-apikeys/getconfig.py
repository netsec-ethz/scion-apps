import webapp2
import json

import visconfig


class GetConfig(webapp2.RequestHandler):

    def post(self):

        self.response.headers['Content-Type'] = 'application/json'

        query = visconfig.VisConfig.all()
        num = query.count()
        items = query.fetch(num)
        obj = {}
        for item in items:
            obj[item.lookuptag] = item.value

        self.response.out.write(json.dumps(obj))


app = webapp2.WSGIApplication([
    ('/getconfig', GetConfig),
], debug=True)
