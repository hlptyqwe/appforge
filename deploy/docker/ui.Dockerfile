FROM node:24-alpine AS builder

ARG VITE_APP_MODE=admin
ARG VITE_APP_NAME=AppForge
ENV VITE_APP_MODE=${VITE_APP_MODE}
ENV VITE_APP_NAME=${VITE_APP_NAME}

WORKDIR /src
COPY appforge-ui/package.json appforge-ui/package-lock.json ./
RUN npm ci

COPY appforge-ui ./
RUN npm run build

FROM nginx:1.30.4-alpine
COPY deploy/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/dist /usr/share/nginx/html

EXPOSE 80
