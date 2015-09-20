Hi!

I used Go lang and MySQL as storage to complete this task.

You can run my application using docker. I have created container with mysql and my app (for simplifying deployment)

You can build docker container like this (in my application path):

```
docker build -t gotest .
docker run -p 8080:8080 -v (pwd):/go gotest
```

Or you can pull my prepared container

```
docker run -p 8080:8080 -v `(pwd)`:/go metalguardian/gotest
```

Then you can go to the browser [http://localhost:8080/](http://localhost:8080/)
