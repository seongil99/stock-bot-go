mongo:
	docker run -d -p 27017:27017 --name mongo \
	-e MONGO_INITDB_ROOT_USERNAME=root \
	-e MONGO_INITDB_ROOT_PASSWORD=example \
	mongo:8.0.4