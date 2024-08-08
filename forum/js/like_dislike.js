// Event Listener for interacting with loaded content (post)
document.addEventListener("DOMContentLoaded", function() {
    console.log("DOM fully loaded and parsed");

    // Target the div content with class 'post'
    var postElements = document.getElementsByClassName('post');

    // For each post
    for (var i = 0; i < postElements.length; i++) {
        // Assign current post to a variable and get the ID from the data-post-id attribute
        var postElement = postElements[i];
        var postId = postElement.getAttribute('data-post-id');
        var topId = postElement.getAttribute('data-top-id')
        
        // Get the button elements from thread.html
        var likeButton = postElement.querySelector('.like');
        var dislikeButton = postElement.querySelector('.dislike');
        
        // Functions for when the above button elements are clicked
        likeButton.onclick = function(postId, topId) {
            return function() {
                likePost(postId, topId);
            };
        }(postId, topId);
        
        dislikeButton.onclick = function(postId, topId) {
            return function() {
                dislikePost(postId, topId);
            };
        }(postId, topId);
    }
});

// What to do when a post is liked
function likePost(postId, topId) {
    fetch(`/like/${topId}/${postId}`, {
        method: 'POST'
    })

    // Send response to server
    .then(response => response.json())
    
    // Receive json data back from server
    .then(data => {
        document.getElementById(`likes-${postId}`).innerText = data.likes;
        document.getElementById(`dislikes-${postId}`).innerText = data.dislikes;
    })

    // Catch any errors
    .catch(error => console.error('Error:', error));
}

// What to do when a post is disliked
function dislikePost(postId, topId) {
    fetch(`/dislike/${topId}/${postId}`, {
        method: 'POST'
    })
    
    // Send response to server
    .then(response => response.json())
    
    // Receive json data back from server
    .then(data => {
        document.getElementById(`likes-${postId}`).innerText = data.likes;
        document.getElementById(`dislikes-${postId}`).innerText = data.dislikes;
    })

    // Catch any errors
    .catch(error => console.error('Error:', error));
}
