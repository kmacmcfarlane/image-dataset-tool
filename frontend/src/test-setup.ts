// Global test setup for Vitest
// Per TEST_PRACTICES.md: localStorage isolation
beforeEach(() => {
  localStorage.clear()
})
