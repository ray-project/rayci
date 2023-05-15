// Define patterns
const filePattern = /^[a-zA-Z0-9_/.-]+$/;
const uriPattern = /^https?:\/\/[a-zA-Z0-9._:/&#-]+$/;

function deserializeIntoArray(string, delimiter="||") {
  return string.split(delimiter).filter(item => item.trim() !== "");
}

function parseTrackedFilesToURIs(content) {
  let lines = content.split("\n");
  let filesToExternalUri = {};

  for (let line of lines) {
    // Skip comments and empty lines
    if (line.startsWith("#") || line.trim() === "") {
      continue;
    }

    let [file, uri] = line.split(" ").map(s => s.trim());

    // Check that these conform to expected patterns
    if (!filePattern.test(file) || !uriPattern.test(uri)) {
      throw new Error(`Invalid file path or URI detected: ${file} ${uri}`);
    }

    filesToExternalUri[file] = uri;
  }
  return filesToExternalUri;
}

function filterFilesByNames(filesToURIs, filenames) {
  let result = {};
  Object.keys(filesToURIs).filter(file => filenames.includes(file)).forEach(file => {
    result[file] = filesToURIs[file];
  });
  return result;
}

function getCommentContentNotChanged(commentHeader) {
  return `${commentHeader}

A previous version of this PR changed code that is used or cited in  external sources, e.g. blog posts.

It looks like these changes have been reverted or are otherwise not present in this PR anymore. Please still carefully review the changes to make sure code we use in external sources still works.`;
}

function getCommentContentChanged(commentHeader, changedFilesToURIs) {
  var commentBody = `${commentHeader}

This PR changes code that is used or cited in external sources, e.g. blog posts.

Before merging this PR, please make sure that the code in the external sources is still working, and consider updating them to reflect the changes.

The affected files and the external sources are:`;

  for (let file in changedFilesToURIs) {
    const fileMessage = `- \`${file}\`: ${changedFilesToURIs[file]}`;
    commentBody += `\n${fileMessage}`;
  }
  return commentBody;
}

module.exports = {
  deserializeIntoArray,
  filterFilesByNames,
  getCommentContentChanged,
  getCommentContentNotChanged,
  parseTrackedFilesToURIs
};
