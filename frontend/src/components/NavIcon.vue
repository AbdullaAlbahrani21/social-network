<script setup>
const props = defineProps({
  name: { type: String, required: true },
  filled: { type: Boolean, default: false },
})

const PATHS = {
  home: ['M3 10.5 12 3l9 7.5', 'M5 9.5V20a1 1 0 0 0 1 1h4v-6h4v6h4a1 1 0 0 0 1-1V9.5'],
  groups: [
    'M12.5 8a3.5 3.5 0 1 1-7 0 3.5 3.5 0 0 1 7 0Z',
    'M2.5 20a6.5 6.5 0 0 1 13 0',
    'M16 4.6a3.5 3.5 0 0 1 0 6.8',
    'M18.5 14.2A6.5 6.5 0 0 1 21.5 20',
  ],
  messages: ['M21 12a8 8 0 0 1-11.6 7.1L4 20.5l1.4-5A8 8 0 1 1 21 12Z'],
  notifications: ['M6 16v-5a6 6 0 1 1 12 0v5l1.5 2h-15Z', 'M10 21a2 2 0 0 0 4 0'],
  profile: ['M16 8a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z', 'M4 21a8 8 0 0 1 16 0'],
  plus: ['M12 5v14', 'M5 12h14'],
  logout: ['M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4', 'm16 17 5-5-5-5', 'M21 12H9'],
  heart: [
    'M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z',
  ],
  back: ['M19 12H5', 'm12 19-7-7 7-7'],
  search: ['M19 11a8 8 0 1 1-16 0 8 8 0 0 1 16 0Z', 'm21 21-4.3-4.3'],
  lock: ['M6 11h12a1 1 0 0 1 1 1v8a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1v-8a1 1 0 0 1 1-1Z', 'M8 11V7a4 4 0 1 1 8 0v4'],
  globe: [
    'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z',
    'M3 12h18',
    'M12 3a14 14 0 0 1 0 18',
    'M12 3a14 14 0 0 0 0 18',
  ],
  close: ['M18 6 6 18', 'm6 6 12 12'],
  eye: ['M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z', 'M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z'],
  'eye-off': [
    'M10.6 5.1A10 10 0 0 1 12 5c6.5 0 10 7 10 7a17 17 0 0 1-2.2 3.2',
    'M6.6 6.6A17 17 0 0 0 2 12s3.5 7 10 7a9.7 9.7 0 0 0 5.4-1.6',
    'M9.9 9.9a3 3 0 0 0 4.2 4.2',
    'm2 2 20 20',
  ],
  mail: ['M4 5h16a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1Z', 'm3 7 9 6 9-6'],
  at: ['M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z', 'M16 8v5a3 3 0 0 0 6 0v-1a10 10 0 1 0-3.9 7.9'],
  calendar: ['M5 5h14a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1Z', 'M16 3v4', 'M8 3v4', 'M4 10h16'],
  text: ['M4 6h16', 'M4 12h16', 'M4 18h10'],
  image: [
    'M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Z',
    'M11 9a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z',
    'm21 15-3.1-3.1a2 2 0 0 0-2.8 0L6 21',
  ],
}
</script>

<template>
  <svg
    class="nav-icon"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    :fill="props.filled ? 'currentColor' : 'none'"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path v-for="d in PATHS[props.name]" :key="d" :d="d" />
  </svg>
</template>

<style scoped>
.nav-icon {
  flex: none;
  display: block;
}
</style>
