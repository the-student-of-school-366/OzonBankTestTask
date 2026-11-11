FROM ubuntu:latest
LABEL authors="Anand"

ENTRYPOINT ["top", "-b"]