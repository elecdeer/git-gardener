FROM node:26-slim

RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*
RUN corepack enable

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .

RUN pnpm run build

CMD ["pnpm", "run", "test"]
