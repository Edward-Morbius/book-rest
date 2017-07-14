#!/bin/sh
curl http://localhost:8080/pages
curl -X PUT -d @update.json  http://localhost:8080/pages
curl -X DELETE http://localhost:8080/pages/0
curl -X UPDATE -d @update.json  http://localhost:8080/pages/0
