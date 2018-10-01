# hiprice-dispatcher
Dispatcher for HiPrice.

## Build Docker Image
```
docker build -f Dockerfile -t hiprice-dispatcher .

// if you do not want to build yourself, a default image is ready in use
docker pull wf2030/hiprice-dispatcher:0.1.0
```

## Run In Docker
`docker run -d --name hiprice-dispatcher --link mariadb:mariadb --link beanstalk:beanstalk hiprice-dispatcher`
