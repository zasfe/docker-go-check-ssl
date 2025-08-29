# --- 1단계: 빌드 환경 ---
# Go 언어 공식 이미지를 빌더(builder)로 사용합니다.
FROM golang:1.21-alpine AS builder

# 작업 디렉토리 설정
WORKDIR /app

# go.mod와 go.sum 파일을 먼저 복사하여 의존성 레이어를 캐시합니다.
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 전체를 복사합니다.
COPY . .

# 애플리케이션을 빌드합니다.
# CGO_ENABLED=0: C 라이브러리 의존성 없이 정적 바이너리 생성
# -ldflags="-w -s": 디버깅 정보 제거하여 바이너리 크기 축소
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s" -o /main .

# --- 2단계: 실행 환경 ---
# 매우 가벼운 Alpine Linux 이미지를 기반으로 최종 이미지를 만듭니다.
FROM alpine:latest

# SSL/TLS 검증에 필요한 루트 CA 인증서를 설치합니다.
RUN apk --no-cache add ca-certificates

# 작업 디렉토리 설정
WORKDIR /root/

# 빌드 단계에서 생성된 실행 파일만 복사합니다.
COPY --from=builder /main .

# 컨테이너가 시작될 때 실행할 명령을 지정합니다.
CMD ["./main"]

