//Go 在查询仓库版本信息时，将并发的请求合并成 1 个请求:
func metaImportsForPrefix(importPrefix string, mod ModuleMode, security web.SecurityMode) (*urlpkg.URL, []metaImport, error) {
	// 使用缓存保存请求结果
setCache := func(res fetchResult) (fetchResult, error) {
  fetchCacheMu.Lock()
  defer fetchCacheMu.Unlock()
  fetchCache[importPrefix] = res
  return res, nil

	// 使用 SingleFlight请求
resi, _, _ := fetchGroup.Do(importPrefix, func() (resi interface{}, err error) {
  fetchCacheMu.Lock()
		// 如果缓存中有数据，那么直接从缓存中取，这也是常用的一种解决缓存击穿的例子。
  if res, ok := fetchCache[importPrefix]; ok {
	fetchCacheMu.Unlock()
	return res, nil
  }
  fetchCacheMu.Unlock()
		......