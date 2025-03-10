# Architecture Diagrams

* [`yggd-cancel-command.mmd`](./yggd-cancel-command.mmd) diagrams the data flow
  on how a worker that supports cancellation can cancel a data request.
* [`yggd-connection.mmd`](./yggd-connection.mmd) diagrams the connection/startup
  flow between `yggd`, workers and the MQTT broker.
* [`yggd-data.mmd`](./yggd-data.mmd) diagrams how data destined for a worker
  flows through MQTT, `yggd`, and D-Bus to the worker.
