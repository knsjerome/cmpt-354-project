import Vue from 'vue';
import VueRouter from 'vue-router';
import store from '@/store/index';
import Home from '../views/Home.vue';

Vue.use(VueRouter);

const routes = [
  {
    path: '/',
    name: 'Home',
    component: Home,
    meta: {
      layout: 'Main',
      protected: true,
    },
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('../views/Registration.vue'),
    meta: {
      layout: 'Guest',
    },
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
    meta: {
      layout: 'Guest',
    },
  },
  {
    path: '/campaign',
    name: 'Campaign',
    component: () => import('../views/Campaign.vue'),
    meta: {
      layout: 'Main',
      protected: true,
    },
  },
  {
    /* TODO: make protected route */
    path: '/characters',
    name: 'Characters',
    component: () => import('../views/Characters.vue'),
    meta: {
      layout: 'Main',
    },
  },
  {
    /* TODO: make protected route */
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('../views/Dashboard.vue'),
    meta: {
      layout: 'Main',
    },
  },
];

const router = new VueRouter({
  routes,
});

router.beforeEach(async (to, _, next) => {
  const isProtectedRoute = to.matched.some((record) => record.meta.protected);
  const isPlayerLoggedIn = store.getters['authentication/isPlayerLoggedIn'];
  const playerCanAccess = !isProtectedRoute || (isProtectedRoute && isPlayerLoggedIn);

  if (playerCanAccess) next();
  else next('/login');
});

export default router;
