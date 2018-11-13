FROM alpine:latest
MAINTAINER Alexander Zaytsev "thebestzorro@yandex.ru"
RUN apk update && \
    apk upgrade && \
    apk add ca-certificates tzdata
ADD enigma /bin/enigma
RUN chmod 0755 /bin/enigma
EXPOSE 8082
VOLUME ["/data/conf/"]
ENTRYPOINT ["enigma"]
CMD ["-config", "/data/conf/enigma.json"]
