/* @meta
{
  "name": "hackernews/top",
  "description": "Hacker News top stories (via browser, no auth needed)",
  "domain": "news.ycombinator.com",
  "args": {
    "limit": {"required": false, "description": "Number of stories to return", "default": "10"}
  },
  "readOnly": true,
  "example": "anyclaw site hackernews/top --limit 5"
}
*/
async function(args) {
  const limit = parseInt(args.limit) || 10;
  const ids = await fetch("https://hacker-news.firebaseio.com/v0/topstories.json").then(r => r.json());
  const top = ids.slice(0, limit);
  const stories = await Promise.all(
    top.map(id => fetch("https://hacker-news.firebaseio.com/v0/item/" + id + ".json").then(r => r.json()))
  );
  return stories.map(s => ({
    title: s.title,
    score: s.score,
    url: s.url || "https://news.ycombinator.com/item?id=" + s.id
  }));
}
