module app

go 1.24.5

require github.com/gorilla/websocket v1.5.3

require deploy-scripts v0.0.0

replace deploy-scripts => ../deploy-scripts

replace ocd-scripts => ../ocd-scripts
