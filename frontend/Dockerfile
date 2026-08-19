FROM node:22-alpine AS builder

WORKDIR /build

COPY package.json package-lock.json ./
COPY packages ./packages
RUN npm ci

COPY . .

ARG VITE_FRONTEND_COMMIT=unknown
ENV VITE_FRONTEND_COMMIT=${VITE_FRONTEND_COMMIT}
ENV VITE_IS_DOCKER=true

RUN npm run build

FROM nginx:stable-alpine

COPY --from=builder /build/dist /usr/share/nginx/html

COPY nginx.conf /etc/nginx/templates/default.conf.template

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENV MAX_FILE_SIZE_MB=50

EXPOSE 80

ENTRYPOINT ["/docker-entrypoint.sh"]
