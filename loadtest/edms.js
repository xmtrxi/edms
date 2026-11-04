// Load test for the EDMS API (DOC-046). Run against a staging install, never
// production: it creates folders and documents.
//
//   k6 run -e BASE_URL=https://edms.example.com/api -e EMAIL=load@example.com -e PASSWORD=... loadtest/edms.js
//
// Or authenticate with a personal access token that has read and write scope:
//   k6 run -e BASE_URL=... -e TOKEN=edms_pat_... loadtest/edms.js
//
// PROFILE=smoke (default), load or stress picks the traffic shape. The test
// fails when its thresholds are missed, so it can gate a release.
import http from "k6/http";
import { check, fail, group, sleep } from "k6";
import { Counter, Trend } from "k6/metrics";

const BASE = (__ENV.BASE_URL || "http://localhost:8080/api").replace(/\/$/, "");
const PROFILE = __ENV.PROFILE || "smoke";
const FOLDER_DEPTH = Number(__ENV.FOLDER_DEPTH || 12);
const UPLOAD_KB = Number(__ENV.UPLOAD_KB || 256);
const LARGE_UPLOAD_MB = Number(__ENV.LARGE_UPLOAD_MB || 0);

const profiles = {
  smoke: { browse: [{ duration: "30s", target: 2 }], upload: [{ duration: "30s", target: 1 }] },
  load: {
    browse: [
      { duration: "2m", target: 50 },
      { duration: "5m", target: 50 },
      { duration: "1m", target: 0 },
    ],
    upload: [
      { duration: "2m", target: 10 },
      { duration: "5m", target: 10 },
      { duration: "1m", target: 0 },
    ],
  },
  stress: {
    browse: [
      { duration: "2m", target: 100 },
      { duration: "3m", target: 250 },
      { duration: "2m", target: 0 },
    ],
    upload: [
      { duration: "2m", target: 20 },
      { duration: "3m", target: 50 },
      { duration: "2m", target: 0 },
    ],
  },
};
const shape = profiles[PROFILE] || fail(`unknown PROFILE ${PROFILE}`);

export const options = {
  scenarios: {
    browse: { executor: "ramping-vus", exec: "browse", stages: shape.browse },
    upload: { executor: "ramping-vus", exec: "upload", stages: shape.upload },
  },
  thresholds: {
    http_req_failed: ["rate<0.01"],
    "http_req_duration{kind:read}": ["p(95)<500", "p(99)<1500"],
    "http_req_duration{kind:search}": ["p(95)<1000"],
    "http_req_duration{kind:write}": ["p(95)<3000"],
    deep_folder_listing: ["p(95)<800"],
  },
};

const deepFolderListing = new Trend("deep_folder_listing", true);
const uploads = new Counter("uploads");

const json = (res) => {
  try {
    return res.json();
  } catch {
    return null;
  }
};

function headers(token, extra = {}) {
  return { headers: { Authorization: `Bearer ${token}`, ...extra } };
}

// setup signs in once (sign-in is rate limited) and builds a deep folder
// tree so permission inheritance is exercised on every listing.
export function setup() {
  let token = __ENV.TOKEN;
  if (!token) {
    const res = http.post(`${BASE}/auth/login`, JSON.stringify({ email: __ENV.EMAIL, password: __ENV.PASSWORD }), {
      headers: { "Content-Type": "application/json" },
    });
    const body = json(res);
    if (res.status !== 200 || !body?.data?.token) {
      fail(`sign-in failed (${res.status}); two-step sign-in accounts need TOKEN instead`);
    }
    token = body.data.token;
  }

  const runId = `k6-${Date.now()}`;
  let parent = "";
  const chain = [];
  for (let depth = 0; depth < FOLDER_DEPTH; depth++) {
    const res = http.post(
      `${BASE}/folders`,
      JSON.stringify({ name: `${runId}-${depth}`, parent_id: parent }),
      headers(token, { "Content-Type": "application/json" }),
    );
    const id = json(res)?.data?.id;
    if (res.status !== 201 || !id) fail(`creating folder level ${depth} failed (${res.status}): ${res.body}`);
    chain.push(id);
    parent = id;
  }
  return { token, folders: chain, deepest: parent, runId };
}

const searchTerms = ["invoice", "contract", "2026", "budget", "policy", "report"];

export function browse(data) {
  const auth = headers(data.token);
  group("browse", () => {
    const list = http.get(`${BASE}/documents?page_size=25`, { ...auth, tags: { kind: "read", name: "documents" } });
    check(list, { "documents listed": (r) => r.status === 200 });
    const docs = json(list)?.data ?? [];

    if (docs.length) {
      const doc = docs[Math.floor(Math.random() * docs.length)];
      const one = http.get(`${BASE}/documents/${encodeURIComponent(doc.id)}`, { ...auth, tags: { kind: "read", name: "document" } });
      check(one, { "document opened": (r) => r.status === 200 });
      http.get(`${BASE}/documents/${encodeURIComponent(doc.id)}/versions`, { ...auth, tags: { kind: "read", name: "versions" } });
    }

    const deep = http.get(`${BASE}/folders/${encodeURIComponent(data.deepest)}`, { ...auth, tags: { kind: "read", name: "deep folder" } });
    deepFolderListing.add(deep.timings.duration);
    check(deep, { "deep folder opened": (r) => r.status === 200 });

    const term = searchTerms[Math.floor(Math.random() * searchTerms.length)];
    const search = http.get(`${BASE}/search?q=${term}`, { ...auth, tags: { kind: "search", name: "search" } });
    check(search, { "search answered": (r) => r.status === 200 || r.status === 400 });

    http.get(`${BASE}/notifications?limit=10`, { ...auth, tags: { kind: "read", name: "notifications" } });
  });
  sleep(1 + Math.random() * 2);
}

const smallFile = "%PDF-1.4\n" + "x".repeat(UPLOAD_KB * 1024) + "\n%%EOF\n";
const largeFile = LARGE_UPLOAD_MB > 0 ? "%PDF-1.4\n" + "y".repeat(LARGE_UPLOAD_MB * 1024 * 1024) + "\n%%EOF\n" : null;

export function upload(data) {
  const folder = data.folders[Math.floor(Math.random() * data.folders.length)];
  // A unique trailer keeps duplicate detection from skipping the upload.
  const body = (largeFile && __ITER % 20 === 0 ? largeFile : smallFile) + `${data.runId}-${__VU}-${__ITER}`;
  const res = http.post(
    `${BASE}/documents`,
    {
      file: http.file(body, `load-${__VU}-${__ITER}.pdf`, "application/pdf"),
      folder_id: folder,
    },
    { ...headers(data.token), tags: { kind: "write", name: "upload" } },
  );
  if (check(res, { "upload stored": (r) => r.status === 201 || r.status === 200 })) uploads.add(1);
  sleep(2 + Math.random() * 3);
}

export function teardown(data) {
  // Folders are left for inspection; delete them from the Drive when done.
  console.log(`Created folder chain ${data.runId}-0 … ${data.runId}-${FOLDER_DEPTH - 1}`);
}
