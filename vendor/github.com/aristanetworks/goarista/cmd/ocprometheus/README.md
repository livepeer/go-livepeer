# ocprometheus

This is a client for the OpenConfig gRPC interface that pushes telemetry to
Prometheus.  Non-numerical data isn't supported by Prometheus and is silently
dropped. Arrays (even with numeric values) are not yet supported.

This tool requires a config file to specify how to map the path of the
notificatons coming out of the OpenConfig gRPC interface onto Prometheus
metric names, and how to extract labels from the path.  For example, the
following rule, excerpt from `sampleconfig.yml`:

```yaml
metrics:
        - name: tempSensor
          path: /Sysdb/environment/temperature/status/tempSensor/(?P<sensor>.+)/(?P<type>(?:maxT|t)emperature)/value
          help: Temperature and Maximum Temperature
          ...
```

Applied to an update for the path
`/Sysdb/environment/temperature/status/tempSensor/TempSensor1/temperature/value`
will lead to the metric name `tempSensor` and labels `sensor=TempSensor1` and `type=temperature`.

Basically, named groups are used to extract (optional) metrics.
Unnamed groups will be given labels names like "unnamedLabelX" (where X is the group's position).
The timestamps from the notifications are not preserved since Prometheus uses a pull model and
doesn't have (yet) support for exporter specified timestamps.
Prometheus 2.0 will probably support timestamps.

## Usage

See the `-help` output, but here's an example to push all the metrics defined
in the sample config file:
```
ocprometheus -addrs <switch-hostname>:6042 -config sampleconfig.json
```
