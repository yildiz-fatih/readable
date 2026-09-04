# build stage
FROM node:26.8.1-alpine3.24 AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --omit=dev

# runtime stage
FROM node:26.8.1-alpine3.24

WORKDIR /app

COPY --from=builder /app/node_modules ./node_modules
COPY . .

CMD ["node", "main.js"]
