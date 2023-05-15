// utils.test.js
const assert = require("assert");
const fs = require("fs");
const {
  deserializeIntoArray,
  filterFilesByNames,
  getCommentContentChanged,
  getCommentContentNotChanged,
  parseTrackedFilesToURIs,
  readFileContent
} = require("./track_code");

// Testing deserializeIntoArray function
assert.deepStrictEqual(deserializeIntoArray("1||2||3"), ["1", "2", "3"]);
assert.deepStrictEqual(deserializeIntoArray("1||2||3||", "||"), ["1", "2", "3"]);
assert.deepStrictEqual(deserializeIntoArray("||1||2||3||", "||"), ["1", "2", "3"]);

// Testing readFileContent function
// Create a dummy file and use it for the test
fs.writeFileSync("test.txt", "Hello, World!", "utf8");
assert.strictEqual(readFileContent("test.txt"), "Hello, World!");
fs.unlinkSync("test.txt"); // clean up after test

// Testing parseTrackedFilesToURIs function
const sampleContent = `
# Comment
file1 http://example1.com
file2 http://example2.com
# Another comment
file3 http://example3.com
`;
let expectedArray = {
  "file1": "http://example1.com",
  "file2": "http://example2.com",
  "file3": "http://example3.com"
};
assert.deepStrictEqual(parseTrackedFilesToURIs(sampleContent), expectedArray);

// Testing filterFilesByNames function
const filesToURIs = {
  "file1": "http://example1.com",
  "file2": "http://example2.com",
  "file3": "http://example3.com"
};
const filenames = ["file1", "file3"];
let expectedArray2 = {
  "file1": "http://example1.com",
  "file3": "http://example3.com"
};
assert.deepStrictEqual(filterFilesByNames(filesToURIs, filenames), expectedArray2);

// Testing getCommentContentNotChanged function
const commentHeader = "## Attention: External code changed";
let expectedOutput = `${commentHeader}

A previous version of this PR changed code that is used or cited in  external sources, e.g. blog posts.

It looks like these changes have been reverted or are otherwise not present in this PR anymore. Please still carefully review the changes to make sure code we use in external sources still works.`;
assert.strictEqual(getCommentContentNotChanged(commentHeader), expectedOutput);

// Testing getCommentContentChanged function
const changedFilesToURIs = {
  "file1": "http://example1.com",
  "file3": "http://example3.com"
};
let expectedOutput2 = `${commentHeader}

This PR changes code that is used or cited in external sources, e.g. blog posts.

Before merging this PR, please make sure that the code in the external sources is still working, and consider updating them to reflect the changes.

The affected files and the external sources are:
- \`file1\`: http://example1.com
- \`file3\`: http://example3.com`;
assert.strictEqual(getCommentContentChanged(commentHeader, changedFilesToURIs), expectedOutput2);
