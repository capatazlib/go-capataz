FROM ubuntu:bionic
RUN mkdir -p /app/run
ADD ./monitoring /app/run
ENTRYPOINT ["/app/run/monitoring"]
