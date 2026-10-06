# --- build stage ---
#
# node:22, not node:20: vite 8 and @vitejs/plugin-vue 6 both declare
# engines ^20.19.0 || >=22.12.0, and the node:20 tag floats across the whole
# 20.x line including releases below 20.19.
FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .

# Vite bakes this in at build time. Pass it via docker-compose build args.
#
# Empty by default, and that is the intended value behind nginx: api.js and
# utils/display.js interpolate it as `${BASE_URL}${path}`, so an empty string
# makes every API call and image URL relative to whatever origin served the
# page. nginx then proxies /api, /uploads and /ws to the backend, and the
# whole app is same-origin.
#
# There is no VITE_WS_URL. stores/websocket.js builds its URL from
# window.location.host and never read one.
ARG VITE_API_BASE_URL=""
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

RUN npm run build

# --- serve stage ---
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
