protoc-setup:
	cd meshes
	wget https://raw.githubusercontent.com/layer5io/meshery/master/meshes/meshops.proto

proto:	
	protoc -I meshes/ meshes/meshops.proto --go_out=plugins=grpc:./meshes/

docker:
	DOCKER_BUILDKIT=1 docker build -t meshery/meshery-tanzu-sm .

docker-run:
	(docker rm -f meshery-tanzu-sm) || true
	docker run --name meshery-tanzu-sm -d \
	-p 10000:10000 \
	-e DEBUG=true \
	meshery/meshery-tanzu-sm

run:
	DEBUG=true GOPROXY=direct GOSUMDB=off go run main.go