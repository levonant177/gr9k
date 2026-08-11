<script>
  import '../app.css';
</script>
<div class="min-h-screen flex flex-col">
  <header class="glass sticky top-0 z-50 border-b border-tg-border">
    <div class="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 rounded-lg bg-tg-accent flex items-center justify-center text-white font-bold text-sm">UPS</div>
        <span class="font-semibold hidden sm:block">ECO-SYSTEM</span>
      </div>
      <nav class="hidden md:flex gap-1 text-sm">
        <a href="/" class="px-3 py-1.5 rounded-lg text-tg-textSecondary hover:bg-black/5">Дашборд</a>
        <a href="/items" class="px-3 py-1.5 rounded-lg text-tg-textSecondary hover:bg-black/5">Номенклатура</a>
        <a href="/warehouse" class="px-3 py-1.5 rounded-lg text-tg-textSecondary hover:bg-black/5">Склад</a>
      </nav>
    </div>
  </header>
  <main class="flex-1 max-w-7xl w-full mx-auto px-4 py-6"><slot /></main>
</div>
