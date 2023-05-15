const {
  deserializeIntoArray,
  filterFilesByNames,
  getCommentContentChanged,
  getCommentContentNotChanged,
  parseTrackedFilesToURIs,
  readFileContent
} = require('./track_code');

const fs = require("fs");

const commentHeader = `## Attention: External code changed`
const externalCodeFile = "doc/external/external_code.txt"

// Get existing comments
const existingComments = await github.rest.issues.listComments({
  owner: context.repo.owner,
  repo: context.repo.repo,
  issue_number: context.issue.number
});

// Find comment by the bot that starts with the header
let commentToUpdate = existingComments.data.find(comment =>
  comment.user.login === 'github-actions[bot]' && comment.body.startsWith(commentHeader)
);

// Read and parse external_code.txt file
let externCodeFileContent = fs.readFileSync(externalCodeFile, "utf8");
let trackedFilesToURIs = parseTrackedFilesToURIs(externCodeFileContent);

console.log("trackedFileToURIs")
console.log(trackedFilesToURIs)

// Get changed files from environment variable
let changedFiles = await deserializeIntoArray(process.env.GIT_DIFF_SERIALIZED)

console.log("changedFiles")
console.log(changedFiles)

// Filter associative array
let changedFileToURIs = filterFilesByNames(trackedFilesToURIs, changedFiles);

console.log("changedFileToURIs")
console.log(changedFileToURIs)

if (changedFileToURIs.length === 0) {
  if (commentToUpdate) {
      commentBody = getCommentContentNotChanged(commentHeader);
      await github.rest.issues.updateComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        comment_id: commentToUpdate.id,
        body: commentBody
      });
  }

} else {
    commentBody = getCommentContentChanged(commentHeader, changedFileToURIs);

    if (commentToUpdate) {
    // Only update if content changed
    if (commentBody !== commentToUpdate.body) {
      await github.rest.issues.updateComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        comment_id: commentToUpdate.id,
        body: commentBody
      });
    }
  } else {
    await github.rest.issues.createComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: context.issue.number,
      body: commentBody
    });
  }
  await github.rest.issues.addLabels({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: context.issue.number,
    labels: ['external-code-affected']
  });

}
