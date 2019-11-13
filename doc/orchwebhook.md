# Orchestrator Webhook

Livepeer supports orchestrator discovery using a webhook. Webhook orchestrator discovery can be
enabled by starting a Livepeer Broadcaster node with the `-orchWebhookUrl <endpoint>` flag.
The `<endpoint>` is a url that returns an array of objects that each contains an "address" key
and a URL string value.

For example:

```json
[
    {"address":"https://10.4.3.2:8935"},
    {"address":"https://10.4.4.3:8935"},
    {"address":"https://10.4.5.2:8935"}
]
```

The orchestrator webhook allows a Broadcaster node operator to periodically refresh its list of available orchestrators. 
The list is refreshed no more than once per minute or as needed, depending on streaming conditions. Refer to the [reliability documentation](https://github.com/livepeer/go-livepeer/blob/master/doc/reliability.md) for more information.
