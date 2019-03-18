# RTMP Webhook Authentication

Incoming RTMP streams can be authenticated using Webhooks. To use it you need to implement own web server which will be called by Livepeer node when new RTMP stream received to check if that stream is allowed.

To enable Webhook authentication functionality Livepeer node should be started with flag `-authWebhookUrl`, like this:

`livepeer -authWebhookUrl http://ownserver/auth`

For every incoming RTMP stream Livepeer node will make `POST` request to `http://ownserver/auth` endpoint passing url to which RTMP request was made as JSON object.

For example, if incoming RTMP request was made to `rtmp://livepeer.node:1935/something?manifestID=manifest`, Liverpeer node will pass this object in request to webhook:

```json
{
    "url": "rtmp://livepeer.node:1935/something?manifestID=manifest"
}
```

Webhook server should respond with code 200 if it wants to allow RTMP stream. Response with any other code will lead to Livepeer node dropping RTMP stream.

Webhook can respond with zero body - in that case `ManifestID` for the stream will be taken from url or generated randomly in case manifest id is not specified in url. Or webhook server can respond with JSON object in this format:

```json
{
    "manifestID": "StrigThatIsIDofManifest"
}
```
and Livepeer node will use returned `ManifestID` for the stream.

`ManifestID`, returned from webhook or provided in url shouldn't have backslahes in it, any other alpha-numeric characters allowed in url will do.

There is simple webhook authentication server [example](https://github.com/livepeer/go-livepeer/blob/master/cmd/simple_auth_server/simple_auth_server.go).
