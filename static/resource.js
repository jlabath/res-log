$(function(){
    //Res model for our resource
    var ResModel = Backbone.Model.extend({
        defaults: function() {
            return {
                fetchdate: "missing date",
                hookdate: "missing date",
                sha1: "missing checksum",
                resource: {}
            };
        }
    });

    var ResourceView = Backbone.View.extend({
        template: _.template($('#res-template').html()),
        render: function() {
            this.$el.html(this.template(this.model.toJSON()));
            return this;
        }
    });

    //our list based on the search
    var ResList = Backbone.Collection.extend({
         model: ResModel,
         comparator: function(x, y){
            if (x < y) {
                return 1;
            } else if (x > y) {
                return -1;
            }
            return 0;
         }
    });

    //defs for history
    var HistoryItem = Backbone.Model.extend({
        defaults: function() {
            return {
                label: "Departures: -1",
                rid: "-1",
                rtype: "departures"
            };
        }
    });

    var HistoryCollection = Backbone.Collection.extend({
         model: HistoryItem
    });

    var rhistory = new HistoryCollection();
    var HistoryView = Backbone.View.extend({
        template: _.template($('#res-history').html()),
        el: $("#history"),
        render: function() {
            this.$el.html(this.template(this));
        },
        initialize: function() {
            this.listenTo(rhistory, 'add', this.render);
        },
        events: {
            "click li": "loadHi"
        },
        loadHi: function(evt){
            var clickedLi = $(evt.target);
            var hi = this.collection.at(clickedLi.index());
            $("#restype").val(hi.get("rtype"));
            $("#resid").val(hi.get("rid"));
            $("#resgo").trigger($.Event("click"));
        }
    });

    var resources = new ResList();
    var hview = new HistoryView({collection: rhistory});
    //our view that listens to changes in resources
    var AppView = Backbone.View.extend({
        el: $("#resapp"),
        events: {
            "change #reslst": "showResource",
            "click #resgo": "fetchResources",
            "keypress #resform": "goOnEnter"
        },
        fetchResources: function(){
            var restype = this.$("#restype");
            var resid = this.$("#resid");
            if ($.trim(resid.val())){
                var url = "/l/"+restype.val()+"/"+$.trim(resid.val());
                resources.url = url;
                resources.fetch({reset:true});
            }
        },
        initialize: function() {
            this.listenTo(resources, 'reset', this.render);
        },
        render: function(){
            var lst = this.$("#reslst");
            lst.empty();
            resources.forEach(function(val){
                lst.append($("<option>").html(val.get("fetchdate")).attr('value',val.cid));
            });
            if (resources.length >  0){
                this.$("#reslst").change();
                //we had results add this restype and resid and label to history
                var restype = this.$("#restype");
                var resid = this.$("#resid");
                var h = new HistoryItem({
                    label: restype.find("option:selected").text()+": "+resid.val(),
                    rid: resid.val(),
                    rtype: restype.val()
                });
                if (rhistory.where({rid: h.get("rid"), rtype: h.get("rtype")}).length === 0){
                    rhistory.add(h, {at: 0});
                }
            } else {
                lst.append($("<option>").html("No results found for: " + resources.url.substring(3) + " ").attr('value',''));
                this.$("#resview").hide();
            }
        },
        showResource: function(){
            var cid = this.$("#reslst").val();
            if (cid){
                var rsc = resources.get(cid);
                var view = new ResourceView({model: rsc});
                this.$("#resview").hide();
                this.$("#resview").empty();
                this.$("#resview").append(view.render().el);
                this.$("#resview").show();
            } else {
                this.$("#resview").hide();
            }
        },
        goOnEnter: function(e){
            if (e.keyCode == 13){
                e.preventDefault();
                this.fetchResources();
                return false;
            }
        }
    });
    //start it
    var app = new AppView();
});
