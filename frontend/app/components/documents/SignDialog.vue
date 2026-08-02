<script setup lang="ts">
  import { PenLineIcon, EraserIcon, StampIcon } from "@lucide/vue";
  import { ref, onMounted } from "vue";
  import { toast } from "vue-sonner";

  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    DialogFooter,
  } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
  import {
    addSignature,
    addStamp,
    currentUser,
    STAMP_META,
    type StampKind,
  } from "@/lib/office-store";

  const props = defineProps<{ open: boolean; docId: string; docName: string }>();
  const emit = defineEmits<{ "update:open": [value: boolean] }>();

  const me = currentUser();
  const tab = ref("typed");
  const typedName = ref(me.name);
  const font = ref("'Great Vibes', cursive");
  const role = ref(me.role);
  const initialsVal = ref(me.initials);

  const FONTS = [
    { label: "Signature A", family: "'Great Vibes', cursive" },
    { label: "Signature B", family: "'Dancing Script', cursive" },
    { label: "Signature C", family: "'Pacifico', cursive" },
  ];

  let canvasEl: HTMLCanvasElement | null = $ref(null);
  const drawing = { current: false };
  const lastPt = { current: { x: 0, y: 0 } as { x: number; y: number } | null };

  onMounted(() => {
    if (!props.open) return;
    const link = document.getElementById("sig-fonts") as HTMLLinkElement | null;
    if (!link) {
      const l = document.createElement("link");
      l.id = "sig-fonts";
      l.rel = "stylesheet";
      l.href =
        "https://fonts.googleapis.com/css2?family=Dancing+Script:wght@600&family=Great+Vibes&family=Pacifico&display=swap";
      document.head.appendChild(l);
    }
  });

  const clearCanvas = () => {
    const c = canvasEl;
    if (!c) return;
    const ctx = c.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, c.width, c.height);
  };

  const drawStart = (e: PointerEvent) => {
    const c = canvasEl!;
    c.setPointerCapture(e.pointerId);
    const rect = c.getBoundingClientRect();
    lastPt.current = { x: e.clientX - rect.left, y: e.clientY - rect.top };
    drawing.current = true;
  };

  const drawMove = (e: PointerEvent) => {
    if (!drawing.current) return;
    const c = canvasEl!;
    const ctx = c.getContext("2d")!;
    const rect = c.getBoundingClientRect();
    const x = e.clientX - rect.left,
      y = e.clientY - rect.top;
    ctx.strokeStyle = "#0f172a";
    ctx.lineWidth = 2.2;
    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    ctx.beginPath();
    ctx.moveTo(lastPt.current!.x, lastPt.current!.y);
    ctx.lineTo(x, y);
    ctx.stroke();
    lastPt.current = { x, y };
  };

  const drawEnd = () => {
    drawing.current = false;
    lastPt.current = null;
  };

  const submit = () => {
    if (tab.value === "typed") {
      if (!typedName.value.trim()) return toast.error("Enter your name to sign");
      addSignature({
        docId: props.docId,
        signer: me.name,
        initials: me.initials,
        kind: "typed",
        value: typedName.value.trim(),
        font: font.value,
        role: role.value,
      });
    } else if (tab.value === "draw") {
      const c = canvasEl;
      if (!c) return;
      const data = c.toDataURL("image/png");
      const ctx = c.getContext("2d")!;
      const pixels = ctx.getImageData(0, 0, c.width, c.height).data;
      let inked = false;
      for (let i = 3; i < pixels.length; i += 40)
        if (pixels[i] > 0) {
          inked = true;
          break;
        }
      if (!inked) return toast.error("Please draw your signature");
      addSignature({
        docId: props.docId,
        signer: me.name,
        initials: me.initials,
        kind: "drawn",
        value: data,
        role: role.value,
      });
    } else {
      if (!initialsVal.value.trim()) return toast.error("Enter your initials");
      addSignature({
        docId: props.docId,
        signer: me.name,
        initials: me.initials,
        kind: "initials",
        value: initialsVal.value.toUpperCase().slice(0, 4),
        role: role.value,
      });
    }
    toast.success("Signature added to document");
    emit("update:open", false);
  };
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-w-xl">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2"
          ><PenLineIcon class="text-primary h-4 w-4" /> Sign document</DialogTitle
        >
        <DialogDescription class="truncate">{{ docName }}</DialogDescription>
      </DialogHeader>

      <Tabs v-model="tab">
        <TabsList class="grid w-full grid-cols-4">
          <TabsTrigger value="typed">Type</TabsTrigger>
          <TabsTrigger value="draw">Draw</TabsTrigger>
          <TabsTrigger value="initials">Initials</TabsTrigger>
          <TabsTrigger value="stamp"><StampIcon class="mr-1 h-3.5 w-3.5" /> Stamp</TabsTrigger>
        </TabsList>

        <TabsContent value="typed" class="mt-4 space-y-3">
          <div class="grid gap-1.5">
            <Label class="text-xs">Full name</Label>
            <Input v-model="typedName" />
          </div>
          <div class="grid gap-1.5">
            <Label class="text-xs">Title / role</Label>
            <Input v-model="role" />
          </div>
          <div class="grid gap-1.5">
            <Label class="text-xs">Signature style</Label>
            <div class="grid grid-cols-3 gap-2">
              <button
                v-for="f in FONTS"
                :key="f.family"
                type="button"
                @click="font = f.family"
                :class="`rounded-lg border p-3 text-center transition ${font === f.family ? 'border-primary ring-primary/30 bg-primary/5 ring-2' : 'border-border hover:bg-secondary/60'}`"
              >
                <div class="text-foreground text-2xl" :style="{ fontFamily: f.family }">
                  {{ typedName || "Your name" }}
                </div>
                <div class="text-muted-foreground mt-1 text-[10px]">{{ f.label }}</div>
              </button>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="draw" class="mt-4 space-y-2">
          <div class="border-border bg-secondary/30 rounded-lg border border-dashed p-2">
            <canvas
              ref="canvasEl"
              width="520"
              height="180"
              class="bg-card h-[180px] w-full cursor-crosshair touch-none rounded"
              @pointerdown="drawStart"
              @pointermove="drawMove"
              @pointerup="drawEnd"
              @pointerleave="drawEnd"
            />
          </div>
          <div class="flex items-center justify-between">
            <p class="text-muted-foreground text-[11px]">Sign with mouse, trackpad, or stylus.</p>
            <Button variant="ghost" size="sm" @click="clearCanvas" class="gap-1.5"
              ><EraserIcon class="h-3.5 w-3.5" /> Clear</Button
            >
          </div>
        </TabsContent>

        <TabsContent value="initials" class="mt-4 space-y-3">
          <div class="grid gap-1.5">
            <Label class="text-xs">Initials</Label>
            <Input v-model="initialsVal" maxlength="4" class="tracking-widest uppercase" />
          </div>
          <div class="border-border bg-secondary/40 grid place-items-center rounded-lg border p-6">
            <div
              class="border-primary text-primary grid h-16 w-16 place-items-center rounded-full border-2 text-xl font-semibold tracking-wider"
            >
              {{ initialsVal.toUpperCase().slice(0, 4) || "—" }}
            </div>
          </div>
        </TabsContent>

        <TabsContent value="stamp" class="mt-4">
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="(meta, k) in STAMP_META"
              :key="k"
              type="button"
              @click="
                addStamp(docId, k as StampKind);
                toast.success(`Stamped: ${meta.label}`);
                emit('update:open', false);
              "
              class="border-border hover:bg-secondary/60 flex flex-col items-center gap-2 rounded-lg border p-3 transition"
            >
              <div
                :class="`rounded px-3 py-1.5 ring-2 ${meta.ring} -rotate-6 text-xs font-bold tracking-widest`"
              >
                {{ meta.label }}
              </div>
              <span class="text-muted-foreground text-[11px] capitalize">{{ k }}</span>
            </button>
          </div>
        </TabsContent>
      </Tabs>

      <DialogFooter v-if="tab !== 'stamp'">
        <Button variant="outline" @click="emit('update:open', false)">Cancel</Button>
        <Button @click="submit" class="gap-1.5"
          ><PenLineIcon class="h-3.5 w-3.5" /> Apply signature</Button
        >
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
