# nginx_golang_rmq
This project demonstrates Nginx's reverse proxy based on body parameters of request with the help of Lua Script. REST server is built in golang which also listens on RMQ. Best coding practices is main motto of this project while writing a professional software.
---
## Running the project
1. Change Nginx Config to point to two backend services atleast
2. Run two golang services on different port on same or different machines
3. Run the test file while pushes curl request pointing to Nginx only
4. Verify that load is getting balanced based on body parameter.