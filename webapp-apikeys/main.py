import cgi

from google.appengine.api import users
import webapp2

import visconfig


class MainPage(webapp2.RequestHandler):
    def get(self):
        user = users.get_current_user()

        # only admins registered with this app can request to store data
        if users.is_current_user_admin():

            self.response.out.write("""
                <html>
                <head>
                <style>
                body {
                 font-family: arial, sans-serif;
                }
                table {
                 border-collapse: collapse;
                 width: 100%%;
                }
                td, th {
                 border: 1px solid #ddd;
                 text-align: left;
                 padding: 8px;
                }
                tr:nth-child(even) {
                 background-color: #ddd;
                }
                </style>
                </head>
                <body>
                  <h2>SCION Visualization Administrator Tools</h2>
                  Administrator: %s (%s)<br><br>
            """ % (user.nickname(), user.email()))

            # scan configuration for current stored values and display
            query = visconfig.VisConfig.all()
            num = query.count()
            items = query.fetch(num)
            self.response.out.write("""
                    Current configuration storage<br><table>
                      <tr>
                        <th>lookuptag</th>
                        <th>value</th>
                        <th>comment</th>
                        <th>username</th>
                      </tr>
            """)
            for item in items:
                self.response.out.write("""
                      <tr>
                        <td>%s</td>
                        <td>%s</td>
                        <td>%s</td>
                        <td>%s</td>
                      </tr>
                """ % (item.lookuptag, item.value, item.comment,
                       item.username))
            self.response.out.write("""
                    </table><p>
            """)
            self.response.out.write("""
                     Add/Update Visualization Configuration<br>
                     <form action="/setconfig" method="post">
                        Config Key:<br><textarea rows="1" cols="70"
                            name="lookuptag"></textarea><br>
                        Config Value:<br><textarea rows="2" cols="70"
                            name="vizvalue"></textarea><br>
                        Config Reason:<br><textarea rows="2" cols="70"
                            name="reason"></textarea><br>
                        <div align="left">
                          <p><input type="submit" name="submit"
                              value="Submit Visualization Configuration" /><p>
                        </div>
                      </form>
                     <br>
                </body>
              </html>""")

        else:
            self.redirect(users.create_login_url(self.request.uri))


class SetConfig(webapp2.RequestHandler):
    def post(self):
        user = users.get_current_user()

        # only admins registered with this app can request to store
        # authentication token for the service
        if users.is_current_user_admin():

            lookup = self.request.get('lookuptag')
            value = self.request.get('vizvalue')
            comments = self.request.get('reason')
            user = users.get_current_user()

            # grab latest proper auth token from our cache
            query = visconfig.VisConfig.all()
            query.filter('lookuptag =', lookup)
            num = query.count()

            # store result, updating old one first
            if num == 1:
                credential = query.get()
                credential.value = value
                credential.username = user.email()
                credential.comment = comments
            else:
                credential = visconfig.VisConfig(
                    value=value, username=user.email(), comment=comments,
                    lookuptag=lookup)

            credential.put()
            key = credential.key()
            insertSuccess = True
            if not key.has_id_or_name():
                insertSuccess = False

            # display result
            self.response.out.write('<html><body>')
            self.response.out.write('Config Updated Lookup Tag: ')
            self.response.out.write(cgi.escape(lookup))
            self.response.out.write('<br>')
            self.response.out.write('Config Value: ')
            self.response.out.write(cgi.escape(value))
            self.response.out.write('<br>')
            self.response.out.write('Config Comments: ')
            self.response.out.write(cgi.escape(comments))
            self.response.out.write('<br>')
            self.response.out.write('Update Result: ')
            if insertSuccess:
                self.response.out.write('Success')
            else:
                self.response.out.write('Failed')
            self.response.out.write('</body></html>')

        else:
            self.redirect(users.create_login_url(self.request.uri))


app = webapp2.WSGIApplication([
    ('/', MainPage),
    ('/setconfig', SetConfig),
], debug=True)
