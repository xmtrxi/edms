<script setup lang="ts">
  import { EraserIcon } from "@lucide/vue";
  import { computed, nextTick, shallowRef, watch } from "vue";

  import { Button } from "@/components/ui/button";
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from "@/components/ui/dialog";
  import { Input } from "@/components/ui/input";
  import { Label } from "@/components/ui/label";
  import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

  const open = defineModel<boolean>("open", { required: true });
  const props = withDefaults(
    defineProps<{
      /** Width divided by height of the box the signature fills. */
      aspect?: number;
      kind?: "signature" | "initials";
      name?: string;
    }>(),
    { aspect: 3, kind: "signature", name: "" },
  );
  const emit = defineEmits<{ adopt: [dataUrl: string] }>();

  const scriptFonts = [
    "'Segoe Script', 'Brush Script MT', 'Snell Roundhand', cursive",
    "'Lucida Handwriting', 'Apple Chancery', cursive",
    "'Bradley Hand', 'Comic Sans MS', cursive",
  ];

  const mode = shallowRef<"draw" | "type">("draw");
  const typed = shallowRef("");
  const font = shallowRef(scriptFonts[0]!);
  const canvas = shallowRef<HTMLCanvasElement | null>(null);
  const hasInk = shallowRef(false);
  let drawing = false;
  let last: { x: number; y: number } | null = null;

  const label = computed(() => (props.kind === "initials" ? "initials" : "signature"));
  // A comfortable drawing area with the field's proportions.
  const size = computed(() => {
    const aspect = Math.min(6, Math.max(1, props.aspect));
    const width = 520;
    return { width, height: Math.round(width / aspect) };
  });

  const initialsOf = (value: string) =>
    value
      .split(/\s+/)
      .filter(Boolean)
      .map((part) => part[0]!.toUpperCase())
      .join("")
      .slice(0, 3);

  watch(open, async (value) => {
    if (!value) return;
    typed.value = props.kind === "initials" ? initialsOf(props.name) : props.name;
    hasInk.value = false;
    await nextTick();
    clear();
  });

  const context = () => {
    const element = canvas.value;
    const ctx = element?.getContext("2d");
    if (!element || !ctx) return null;
    return { element, ctx };
  };

  const clear = () => {
    const found = context();
    if (!found) return;
    const ratio = window.devicePixelRatio || 1;
    found.element.width = size.value.width * ratio;
    found.element.height = size.value.height * ratio;
    found.ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    found.ctx.clearRect(0, 0, size.value.width, size.value.height);
    hasInk.value = false;
  };

  const point = (event: PointerEvent) => {
    const rect = canvas.value!.getBoundingClientRect();
    return {
      x: ((event.clientX - rect.left) / rect.width) * size.value.width,
      y: ((event.clientY - rect.top) / rect.height) * size.value.height,
    };
  };

  const start = (event: PointerEvent) => {
    drawing = true;
    last = point(event);
    canvas.value?.setPointerCapture(event.pointerId);
  };

  const move = (event: PointerEvent) => {
    const found = context();
    if (!drawing || !last || !found) return;
    const next = point(event);
    const { ctx } = found;
    ctx.strokeStyle = "#0f172a";
    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    // Faster strokes come out a touch thinner, like ink.
    const speed = Math.hypot(next.x - last.x, next.y - last.y);
    ctx.lineWidth = Math.max(1.6, 3.2 - speed / 18);
    ctx.beginPath();
    ctx.moveTo(last.x, last.y);
    ctx.lineTo(next.x, next.y);
    ctx.stroke();
    last = next;
    hasInk.value = true;
  };

  const stop = () => {
    drawing = false;
    last = null;
  };

  // Renders typed text into a transparent PNG with the field's proportions.
  const renderTyped = () => {
    const element = document.createElement("canvas");
    const scale = 2;
    element.width = size.value.width * scale;
    element.height = size.value.height * scale;
    const ctx = element.getContext("2d")!;
    ctx.scale(scale, scale);
    let fontSize = size.value.height * 0.62;
    ctx.font = `${fontSize}px ${font.value}`;
    const maxWidth = size.value.width * 0.92;
    const measured = ctx.measureText(typed.value).width;
    if (measured > maxWidth) {
      fontSize *= maxWidth / measured;
      ctx.font = `${fontSize}px ${font.value}`;
    }
    ctx.fillStyle = "#0f172a";
    ctx.textBaseline = "middle";
    ctx.textAlign = "center";
    ctx.fillText(typed.value, size.value.width / 2, size.value.height / 2);
    return element.toDataURL("image/png");
  };

  const canAdopt = computed(() =>
    mode.value === "draw" ? hasInk.value : typed.value.trim().length > 0,
  );

  const adopt = () => {
    if (!canAdopt.value) return;
    const url = mode.value === "draw" ? canvas.value!.toDataURL("image/png") : renderTyped();
    emit("adopt", url);
    open.value = false;
  };
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>Add your {{ label }}</DialogTitle>
        <DialogDescription>
          Draw it with your mouse, finger or pen, or type it. It will be placed on the document
          where you clicked.
        </DialogDescription>
      </DialogHeader>

      <Tabs v-model="mode" class="w-full">
        <TabsList class="grid w-full grid-cols-2">
          <TabsTrigger value="draw">Draw</TabsTrigger>
          <TabsTrigger value="type">Type</TabsTrigger>
        </TabsList>

        <TabsContent value="draw" class="space-y-2">
          <div class="relative rounded-lg border bg-white">
            <canvas
              ref="canvas"
              class="block w-full touch-none"
              :style="{ aspectRatio: `${size.width} / ${size.height}` }"
              :aria-label="`Draw your ${label}`"
              @pointerdown="start"
              @pointermove="move"
              @pointerup="stop"
              @pointerleave="stop"
            />
            <div
              class="pointer-events-none absolute inset-x-6 bottom-[22%] border-b border-dashed border-slate-300"
            />
            <p
              v-if="!hasInk"
              class="pointer-events-none absolute inset-0 grid place-items-center text-sm text-slate-400"
            >
              Sign here
            </p>
          </div>
          <div class="flex justify-end">
            <Button variant="ghost" size="sm" class="gap-1.5" :disabled="!hasInk" @click="clear">
              <EraserIcon class="size-3.5" />Clear
            </Button>
          </div>
        </TabsContent>

        <TabsContent value="type" class="space-y-3">
          <div class="space-y-1.5">
            <Label for="typed-signature">Your {{ label }}</Label>
            <Input id="typed-signature" v-model="typed" autocomplete="off" />
          </div>
          <div class="grid gap-2" role="radiogroup" aria-label="Style">
            <button
              v-for="option in scriptFonts"
              :key="option"
              type="button"
              role="radio"
              :aria-checked="font === option"
              class="rounded-lg border bg-white px-4 py-2 text-left text-2xl text-slate-900 transition-colors"
              :class="
                font === option ? 'border-primary ring-primary/30 ring-2' : 'hover:border-slate-400'
              "
              :style="{ fontFamily: option }"
              @click="font = option"
            >
              {{ typed || (kind === "initials" ? "AB" : "Your name") }}
            </button>
          </div>
        </TabsContent>
      </Tabs>

      <p class="text-muted-foreground text-xs">
        By adopting this {{ label }}, you agree it is your legal electronic {{ label }}.
      </p>
      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button :disabled="!canAdopt" @click="adopt">Adopt and sign</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
