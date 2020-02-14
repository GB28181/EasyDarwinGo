<template>
    <div class="container-fluid no-padding">
        <div class="box box-success">
            <div class="box-header">
                <h4 class="text-success text-center">录像列表</h4>           
                <form class="form-inline">
                    <div class="form-group pull-right">
                        <div class="input-group">
                            <input type="text" class="form-control" placeholder="搜索" v-model.trim="q" @keydown.enter.prevent ref="q">
                            <div class="input-group-btn">
                                <button type="button" class="btn btn-default" @click.prevent="doSearch" >
                                    <i class="fa fa-search"></i>
                                </button>  
                            </div>                            
                        </div>
                    </div>                              
                </form>            
            </div>
            <div class="box-body">
                <el-table :data="players" stripe class="view-list" :default-sort="{prop: 'startAt', order: 'descending'}" @sort-change="sortChange">
                    <el-table-column prop="id" label="ID" min-width="120"></el-table-column>
                    <el-table-column prop="path" label="播放地址" min-width="240" show-overflow-tooltip>
                      <template slot-scope="scope">
                        <span>
                          <i class="fa fa-copy" role="button" v-clipboard="scope.row.path" title="点击拷贝" @success="$message({type:'success', message:'成功拷贝到粘贴板'})"></i>
                          {{scope.row.path}}
                          </span>
                      </template>
                    </el-table-column>               
                    <el-table-column prop="transType" label="传输方式" min-width="100"></el-table-column>               
                    <!-- <el-table-column prop="inBytes" label="上行流量" min-width="120" :formatter="formatBytes" sortable="custom"></el-table-column> -->
                    <el-table-column prop="outBytes" label="下行流量" min-width="120" :formatter="formatBytes" sortable="custom"></el-table-column>
                    <el-table-column prop="startAt" label="开始时间" min-width="200" sortable="custom"></el-table-column>
                </el-table>          
            </div>
            <div class="box-footer clearfix" v-if="total > 0">
                <el-pagination layout="prev,pager,next" class="pull-right" :total="total" :page-size.sync="pageSize" :current-page.sync="currentPage"></el-pagination>
            </div>
        </div>               
    </div>
</template>

<script>
import prettyBytes from "pretty-bytes";

import _ from "lodash";
export default {
  props: {},
  data() {
    return {
      code: "",
      msg: "OK",
      data: [],
    };
  },
  beforeDestroy() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = 0;
    }
  },
  mounted() {
    this.$refs["q"].focus();
    this.timer = setInterval(() => {
      this.getPlayers();
    }, 3000);
  },
  watch: {
    q: function(newVal, oldVal) {
      this.doDelaySearch();
    },
    currentPage: function(newVal, oldVal) {
      this.doSearch(newVal);
    }
  },   
  methods: {
    getPlayers() {
      $.get("/api/v1/record").then(data => {
        if (0 === data.code) {
          this.players = data.data;
        }
      });
    },
    doSearch(page = 1) {
      var query = {};
      if (this.q) query["q"] = this.q;
      this.$router.replace({
        path: `/players/${page}`,
        query: query
      });
    },
    doDelaySearch: _.debounce(function() {
      this.doSearch();
    }, 500), 
    sortChange(data) {
      this.sort = data.prop;
      this.order = data.order;
      this.getPlayers();
    },
    formatBytes(row, col, val) {
      if (val == undefined) return "-";
      return prettyBytes(val);
    }
  },
  beforeRouteEnter(to, from, next) {
    next(vm => {
      vm.q = to.query.q || "";
      vm.currentPage = parseInt(to.params.page) || 1;
    });
  },
  beforeRouteUpdate(to, from, next) {
    next();
    this.$nextTick(() => {
      this.q = to.query.q || "";
      this.currentPage = parseInt(to.params.page) || 1;
      this.players = [];
      this.getPlayers();
    });
  }
};
</script>

