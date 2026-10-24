// Date formatting uses the local time zone; pin it so tests are deterministic.
process.env.TZ = "UTC";

import "@testing-library/jest-dom/vitest";
