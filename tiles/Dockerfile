FROM alpine:3.22 AS build
RUN apk add --no-cache git g++ make sqlite-dev zlib-dev bash
RUN git clone --depth 1 --branch 2.79.0 https://github.com/felt/tippecanoe.git /src \
 && cd /src && make -j4 && make install
ENTRYPOINT []
