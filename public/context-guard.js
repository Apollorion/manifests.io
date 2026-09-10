(() => {
  const query = new URLSearchParams(window.location.search);
  if (["path", "linked", "trail"].some(name => query.get(name))) {
    document.documentElement.inert = true;
  }
})();
