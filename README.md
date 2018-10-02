# hiprice-dispatcher
Dispatcher for HiPrice.

## Docker
```
// build
docker image build -f Dockerfile -t hiprice-dispatcher .

// run
docker container run -d --name hiprice-dispatcher --link mariadb:mariadb --link beanstalk:beanstalk hiprice-dispatcher

// if you do not want to build yourself, a default image is ready in use
docker container run -d --name hiprice-dispatcher --link mariadb:mariadb --link beanstalk:beanstalk wf2030/hiprice-dispatcher:0.1.0
```


