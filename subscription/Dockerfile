FROM golang:1.25.5
WORKDIR /usr/src/clusterfuck

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./api ./api
COPY ./subscription ./subscription
RUN go build -v -o /usr/local/bin/subscription ./subscription
EXPOSE 50052
CMD ["subscription"]