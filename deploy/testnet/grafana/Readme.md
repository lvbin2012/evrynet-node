# Run 2 dockers
- access to the dev directory and run the commands here
```
$ docker volume create influxdb-volume
$ docker volume create grafana-volume
$ docker-compose up -d geth-grafana geth-influxdb
```

# Where volume was store
- You need to get volume-id firstly by `docker volume ls | grep <volume-name>`
- Use volume-id above to replace in `/var/lib/docker/volumes/<volume-id>/_data"` 
- Another ways is field "Source" when run `sudo docker inspect --format='{{json .Mounts}}' <docker-name> | python3 -m json.tool`

# Config node to sync the data metrics to influx db
- run node with parameter 

```
--metrics --metrics.influxdb
```

# Import dashboard
- navigate to grafana via url http://localhost:3001
- login to admin site
- config to connect to influxdb with parameters here:
  - endpoint: localhost:8086
  - database: geb
  - user: test
  - password: test
- create dashboard via import method (use EvryNet-dashboard.json)