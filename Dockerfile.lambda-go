FROM golang:1.25 AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE_PATH
RUN CGO_ENABLED=0 GOOS=linux go build -o /build/bootstrap ${SERVICE_PATH}

FROM public.ecr.aws/lambda/provided:al2023

COPY --from=builder /build/bootstrap ${LAMBDA_RUNTIME_DIR}/bootstrap

CMD ["bootstrap"]
