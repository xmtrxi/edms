import { getToken } from "@/utils/token";

export interface RealtimePerson {
  id: string;
  name: string;
  email?: string;
}

export interface RealtimeEvent<T = unknown> {
  type: string;
  topic: string;
  data?: T;
  actor?: RealtimePerson;
  at: number;
}

type Handler = (event: RealtimeEvent) => void;

// One socket per tab, shared by every component that listens.
const handlers = new Map<string, Set<Handler>>();
let socket: WebSocket | null = null;
let retries = 0;
let retryTimer: ReturnType<typeof setTimeout> | undefined;

const send = (message: Record<string, string>) => {
  if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(message));
};

const connect = () => {
  const token = getToken();
  if (!token || socket) return;
  const base = String(useRuntimeConfig().public.apiBase).replace(/\/$/, "");
  const url = new URL(`${base}/realtime`, window.location.href);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  // The token travels as a subprotocol so it stays out of URLs and logs.
  const current = new WebSocket(url, ["edms.v1", token]);
  socket = current;
  current.onopen = () => {
    retries = 0;
    for (const topic of handlers.keys()) {
      if (!topic.startsWith("user:")) send({ op: "subscribe", topic });
    }
  };
  current.onmessage = (message) => {
    try {
      const event = JSON.parse(String(message.data)) as RealtimeEvent;
      for (const handler of handlers.get(event.topic) ?? []) handler(event);
    } catch {
      // Ignore anything that isn't an event.
    }
  };
  current.onclose = () => {
    if (socket === current) socket = null;
    if (!handlers.size) return;
    // Back off up to 30 seconds while the API is unreachable.
    const delay = Math.min(30_000, 1000 * 2 ** retries++);
    clearTimeout(retryTimer);
    retryTimer = setTimeout(connect, delay);
  };
};

/** Live events for documents, folders and the signed-in user. */
export const useRealtime = () => {
  const subscribe = (topic: string, handler: Handler) => {
    if (!import.meta.client) return () => {};
    let set = handlers.get(topic);
    const first = !set;
    if (!set) {
      set = new Set();
      handlers.set(topic, set);
    }
    set.add(handler);
    if (!socket) connect();
    else if (first && !topic.startsWith("user:")) send({ op: "subscribe", topic });
    return () => {
      set.delete(handler);
      if (set.size) return;
      handlers.delete(topic);
      if (!topic.startsWith("user:")) send({ op: "unsubscribe", topic });
      if (!handlers.size) {
        clearTimeout(retryTimer);
        socket?.close();
        socket = null;
      }
    };
  };
  return { subscribe };
};

/** People who have the same topic open right now, not counting yourself. */
export const usePresence = (topic: MaybeRefOrGetter<string>) => {
  const auth = useAuthStore();
  const people = ref<RealtimePerson[]>([]);
  let stop: (() => void) | undefined;
  const { subscribe } = useRealtime();

  const listen = (value: string) => {
    stop?.();
    people.value = [];
    if (!value) return;
    stop = subscribe(value, (event) => {
      const person = event.actor;
      if (!person || person.id === auth.user?.id) return;
      if (event.type === "presence.leave") {
        people.value = people.value.filter((item) => item.id !== person.id);
      } else if (event.type === "presence.join" || event.type === "presence.here") {
        if (!people.value.some((item) => item.id === person.id))
          people.value = [...people.value, person];
      }
    });
  };

  watch(() => toValue(topic), listen, { immediate: true });
  onBeforeUnmount(() => stop?.());
  return people;
};
