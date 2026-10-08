import { defineRailway, preserve, project, service } from "railway/iac";

// This repository manages only its own resources in the environment. Other
// repositories export their own partial name.
// See https://docs.railway.com/infrastructure-as-code#multi-repo-projects
export const partial = "game-server";

export default defineRailway((ctx) => {
  if (ctx.environment !== "staging") {
    throw new Error("FreeTower configuration is restricted to staging");
  }
  const gameServer = service("Game Server", {
    build: { builder: "DOCKERFILE", dockerfilePath: "Dockerfile" },
    deploy: {
      healthcheckPath: "/api/health",
      healthcheckTimeout: 120,
      numReplicas: 1,
      multiRegionConfig: { "asia-southeast1-eqsg3a": { numReplicas: 1 } },
      drainingSeconds: 120,
      overlapSeconds: 0,
      restartPolicyType: "ON_FAILURE",
      restartPolicyMaxRetries: 3,
      sleepApplication: false,
    },
    // Preserve existing Railway-held settings and secrets; never import values.
    env: {
      DATABASE_URL: "${{Postgres.DATABASE_URL}}",
      FREETOWER_ENV: preserve(),
      FREETOWER_REQUIRE_DATABASE: preserve(),
      FREETOWER_PUBLIC_ORIGIN: preserve(),
      FREETOWER_GIT_SHA: preserve(),
      FREETOWER_STAGING_CHARACTER_INSCRIPTION_ID: preserve(),
      FT_CHARACTER_INSCRIPTION_ALLOWLIST: preserve(),
      FT_ENABLE_DEV_IDENTITY: preserve(),
      FT_ENABLE_LEGACY_AUTH: preserve(),
      FT_ENABLE_OWNERSHIP_ACCEPTANCE: preserve(),
      PORT: preserve(),
      UNISAT_FRACTAL_API_KEY: preserve(),
    },
  });
  return project("FreeTower-Staging", {
    resources: [gameServer],
  });
});
