from google.appengine.ext import db


class VisConfig(db.Model):
    # store config values to be recalled
    value = db.TextProperty(required=True)
    lookuptag = db.StringProperty(required=True)
    inserted = db.DateTimeProperty(auto_now=True)
    username = db.StringProperty(required=True)
    comment = db.TextProperty()
