<script setup lang="ts">
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { ref } from 'vue';
import { shortenURL } from './api/endpoints';

const currentOriginalURL = ref('')

const resultOriginalURL = ref('')
const resultShortenedURL = ref('')

async function shorten() {
  if (currentOriginalURL.value.length < 1) {
    return
  }

  const res = await shortenURL({
    url: currentOriginalURL.value
  })


  if (res.error) {
    console.error("failed to shorten url:", res.error)
    return
  }
  if (!res.short_url) {
    console.error("short url is null")
    return
  }

  resultShortenedURL.value = res.short_url
  resultOriginalURL.value = res.original_url
}

</script>

<template>
  <div class="max-w-3xl mx-auto mt-8">
    <Card class="mb-8">
      <CardHeader>
        <CardTitle>Shorten URL</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid items-center w-full gap-4">
          <Label for="url">Link to shorten</Label>
          <Input v-model="currentOriginalURL" id="url" placeholder="https://github.com" />
        </div>
      </CardContent>
      <CardFooter>
        <div class="grid w-full">
          <Button @click="shorten">Shorten</Button>
        </div>
      </CardFooter>
    </Card>

    <Card>
      <CardContent>
        <div class="grid items-center w-full gap-4">
          <p>Original URL: <a :href="resultOriginalURL">{{ resultOriginalURL }}</a></p>
          <p>Shortened URL: <a :href="resultShortenedURL">{{ resultShortenedURL }}</a></p>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
