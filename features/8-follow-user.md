# Follow user

Allow authenticated users to follow and unfollow other users.

## Implementation

There are two new protected endpoints:
POST /api/profiles/:username/follow - follow a user
DELETE /api/profiles/:username/follow - unfollow a user

The logic is that the authenticated user should follow/unfollow the user with username.

A successful response looks like:
```json
{
  "profile": {
    "username": "jake",
    "bio": "I work at statefarm",
    "image": "https://api.realworld.io/images/smiley-cyrus.jpg",
    "following": false
  }
}
```

Note that the following attribute will be true or false depending of it the authenticated user is following the user in the profile of not.

The GET /api/profile/:username endpoint should also be updated to return the correct value of following attribute.

If there is no user with username, return status 404 and
```json
{
  "errors": {
    "profile": ["not found"]
  }
}
```

We need db schema changes so that we can track user follows. Prefer to keep the db schema in 4NF.
