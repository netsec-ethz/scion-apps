# SCIONLab Webapp API Keys

`webapp-apikeys` is a Google App Engine web application designed to store and retrieve API Keys which are not desirable to publish in public open source code repositories. It has been moved from its original location at [https://github.com/netsec-ethz/scion-viz/tree/master/python/appengine](https://github.com/netsec-ethz/scion-viz/tree/master/python/appengine).

This is used to primarily supply API keys from services like Google Maps, Geolocation, Firebase, and others used by SCIONLab visualization tools like [webapp](../webapp).

## Structure

This web application creates a simple key-value paired Datastore, with a permissions-based App Admin Page for it's main page to make editing easy, and one URL for retrieving the JSON-formatted dictionary of key-value pairs using the pattern: `https://[host]/getconfig`.

## Access

Permissions to modify the deployed application have to be granted by a project Owner.

- GAE Project Name: `scion-viz`
- Host & App Admin Page: [https://my-project-1470640410708.appspot.com](https://my-project-1470640410708.appspot.com)
- GAE Permissions: [https://console.cloud.google.com/iam-admin/iam?project=my-project-1470640410708](https://console.cloud.google.com/iam-admin/iam?project=my-project-1470640410708)
- GAE Dashboard: [https://console.cloud.google.com/appengine?project=my-project-1470640410708](https://console.cloud.google.com/appengine?project=my-project-1470640410708)

## Deployment

1. Prepare your development environment: [https://cloud.google.com/appengine/docs/standard/python/setting-up-environment](https://cloud.google.com/appengine/docs/standard/python/setting-up-environment)

1. Read the Google Cloud application upload documentation: [https://cloud.google.com/appengine/docs/standard/python/tools/uploadinganapp](https://cloud.google.com/appengine/docs/standard/python/tools/uploadinganapp)

1. Once you have the tools installed, from the application directory:

    ```
    gcloud app deploy --project my-project-1470640410708
    ```

