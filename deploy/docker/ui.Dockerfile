FROM node:20-alpine AS builder

WORKDIR /src
COPY appforge-ui/package.json appforge-ui/package-lock.json ./
RUN npm ci

COPY appforge-ui ./
RUN npm run build

FROM nginx:1.27-alpine
COPY deploy/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/dist /usr/share/nginx/html

EXPOSE 80

