# Build stage
FROM node:16-alpine as build

ARG REACT_APP_AUTH_URL=http://localhost:8081
ARG REACT_APP_API_URL=http://localhost:8081

ENV REACT_APP_AUTH_URL=$REACT_APP_AUTH_URL
ENV REACT_APP_API_URL=$REACT_APP_API_URL

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# Production stage
FROM nginx:alpine

COPY --from=build /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
