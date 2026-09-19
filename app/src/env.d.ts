// Metro inlines EXPO_PUBLIC_* variables at build time, tech.md section 19.2.
declare const process: {
  env: {
    EXPO_PUBLIC_API_URL?: string;
    EXPO_PUBLIC_DEV_ACCESS_TOKEN?: string;
  };
};
